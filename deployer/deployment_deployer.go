// Package deployer is grid deployer
package deployer

import (
	"context"
	"log"
	"net"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// DeploymentDeployer for deploying a deployment
type DeploymentDeployer struct {
	currentDeployments map[uint64]workloads.Deployment

	TFPluginClient *TFPluginClient
	deployer       Deployer
}

// NewDeploymentDeployer generates a new deployer for a deployment
func NewDeploymentDeployer(tfPluginClient *TFPluginClient) DeploymentDeployer {
	newDeployer := NewDeployer(*tfPluginClient, true)
	deploymentDeployer := DeploymentDeployer{
		currentDeployments: make(map[uint64]workloads.Deployment),
		TFPluginClient:     tfPluginClient,
		deployer:           newDeployer,
	}
	return deploymentDeployer
}

// GenerateVersionlessDeployments generates a new deployment without a version
func (d *DeploymentDeployer) GenerateVersionlessDeployments(ctx context.Context, dl *workloads.Deployment) (map[uint32]gridtypes.Deployment, error) {
	newDl := workloads.NewGridDeployment(d.TFPluginClient.TwinID, []gridtypes.Workload{})
	err := d.assignNodesIPs(dl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	for _, disk := range dl.Disks {
		newDl.Workloads = append(newDl.Workloads, disk.GenerateWorkload())
	}
	for _, zdb := range dl.Zdbs {
		newDl.Workloads = append(newDl.Workloads, zdb.GenerateWorkload())
	}
	for _, vm := range dl.Vms {
		newDl.Workloads = append(newDl.Workloads, vm.GenerateVMWorkload()...)
	}

	for idx, q := range dl.Qsfs {
		qsfsWorkload, err := q.ZosWorkload()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate qsfs %d", idx)
		}
		newDl.Workloads = append(newDl.Workloads, qsfsWorkload)
	}

	return map[uint32]gridtypes.Deployment{dl.NodeID: newDl}, nil
}

// Deploy deploys a new deployment
func (d *DeploymentDeployer) Deploy(ctx context.Context, dl *workloads.Deployment) error {
	if err := d.Validate(ctx, dl); err != nil {
		return err
	}

	newDeployments, err := d.GenerateVersionlessDeployments(ctx, dl)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	if len(dl.SolutionType) == 0 {
		dl.SolutionType = "Virtual Machine"
	}

	deploymentData := workloads.DeploymentData{
		Name:        dl.Name,
		Type:        "vm",
		ProjectName: dl.SolutionType,
	}
	// deployment data
	newDeploymentsData := map[uint32]workloads.DeploymentData{dl.NodeID: deploymentData}

	// solution providers
	newDeploymentsSolutionProvider := map[uint32]*uint64{dl.NodeID: dl.SolutionProvider}

	oldDeployments := d.TFPluginClient.stateLoader.currentNodeDeployment

	currentDeployments, err := d.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)
	if currentDeployments[dl.NodeID] != 0 {
		dl.ContractID = currentDeployments[dl.NodeID]
		d.TFPluginClient.stateLoader.currentNodeDeployment[dl.NodeID] = dl.ContractID
		d.currentDeployments[dl.ContractID] = *dl
	}
	return err
}

// Cancel cancels deployments
func (d *DeploymentDeployer) Cancel(ctx context.Context, dl *workloads.Deployment) error {
	if err := d.Validate(ctx, dl); err != nil {
		return err
	}

	// TODO cancel specific deployment only
	newDeployments := map[uint32]gridtypes.Deployment{}

	// deployment data
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)

	// solution providers
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	oldDeployments := d.TFPluginClient.stateLoader.currentNodeDeployment

	currentDeployments, err := d.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)
	id := currentDeployments[dl.NodeID]
	// TODO: if not cancelled ???
	if id != 0 {
		dl.ContractID = id
	} else {
		dl.ContractID = 0
		delete(d.TFPluginClient.stateLoader.currentNodeDeployment, dl.NodeID)
		delete(d.currentDeployments, dl.ContractID)
	}
	return err
}

// Sync syncs the deployments
func (d *DeploymentDeployer) Sync(ctx context.Context) error {
	currentDeployments, err := d.deployer.GetDeployments(ctx, d.TFPluginClient.stateLoader.currentNodeDeployment)
	if err != nil {
		return errors.Wrap(err, "failed to get deployments to update local state")
	}

	for nodeID, dl := range currentDeployments {
		contractID := d.TFPluginClient.stateLoader.currentNodeDeployment[nodeID]
		contractID, err := d.syncContract(contractID)
		if err != nil {
			return err
		}

		gridDl := d.currentDeployments[contractID]

		if contractID == 0 {
			gridDl.Nullify()
			d.currentDeployments[contractID] = gridDl
			return nil
		}

		var vms []workloads.VM
		var zdbs []workloads.ZDB
		var qsfs []workloads.QSFS
		var disks []workloads.Disk

		network := d.TFPluginClient.stateLoader.networks.getNetwork(gridDl.NetworkName)
		network.deleteDeploymentHostIDs(gridDl.NodeID, gridDl.ContractID)

		usedIPs := []byte{}
		for _, w := range dl.Workloads {
			if !w.Result.State.IsOkay() {
				continue
			}

			switch w.Type {
			case zos.ZMachineType:
				vm, err := workloads.NewVMFromWorkloads(&w, &dl)
				if err != nil {
					log.Printf("error parsing vm: %s", err.Error())
					continue
				}
				vms = append(vms, vm)

				ip := net.ParseIP(vm.IP).To4()
				usedIPs = append(usedIPs, ip[3])

			case zos.ZDBType:
				zdb, err := workloads.NewZDBFromWorkload(&w)
				if err != nil {
					log.Printf("error parsing zdb: %s", err.Error())
					continue
				}

				zdbs = append(zdbs, zdb)
			case zos.QuantumSafeFSType:
				q, err := workloads.NewQSFSFromWorkload(&w)
				if err != nil {
					log.Printf("error parsing qsfs: %s", err.Error())
					continue
				}

				qsfs = append(qsfs, q)

			case zos.ZMountType:
				disk, err := workloads.NewDiskFromWorkload(&w)
				if err != nil {
					log.Printf("error parsing disk: %s", err.Error())
					continue
				}

				disks = append(disks, disk)
			}
		}

		network = d.TFPluginClient.stateLoader.networks.getNetwork(gridDl.NetworkName)
		network.setDeploymentHostIDs(gridDl.NodeID, gridDl.ContractID, usedIPs)

		gridDl.Match(disks, qsfs, zdbs, vms)
		log.Printf("vms: %+v\n", len(vms))

		gridDl.Disks = disks
		gridDl.Qsfs = qsfs
		gridDl.Zdbs = zdbs
		gridDl.Vms = vms
		d.currentDeployments[contractID] = gridDl
	}

	return nil
}

// Validate validates a deployment deployer
func (d *DeploymentDeployer) Validate(ctx context.Context, dl *workloads.Deployment) error {
	sub := d.TFPluginClient.SubstrateConn

	if err := validateAccountBalanceForExtrinsics(sub, d.TFPluginClient.Identity); err != nil {
		return err
	}

	return dl.Validate()
}

func (d *DeploymentDeployer) assignNodesIPs(dl *workloads.Deployment) error {
	znet, err := d.TFPluginClient.stateLoader.LoadNetworkFromGrid(dl.NetworkName)
	if err != nil {
		return errors.Wrapf(err, "error getting network workload")
	}
	ipRange := znet.IPRange.String()

	network := d.TFPluginClient.stateLoader.networks.getNetwork(dl.NetworkName)
	usedHosts := network.getUsedNetworkHostIDs(dl.NodeID)

	if len(dl.Vms) == 0 {
		return nil
	}
	ip, ipRangeCIDR, err := net.ParseCIDR(ipRange)
	if err != nil {
		return errors.Wrapf(err, "invalid ip %s", ipRange)
	}
	for _, vm := range dl.Vms {
		vmIP := net.ParseIP(vm.IP)
		if vmIP != nil {
			vmHostID := vmIP[3]
			if vm.IP != "" && ipRangeCIDR.Contains(vmIP) && !workloads.Contains(usedHosts, vmHostID) {
				usedHosts = append(usedHosts, vmHostID)
			}
		}
	}
	curHostID := byte(2)

	for idx, vm := range dl.Vms {
		if vm.IP != "" && ipRangeCIDR.Contains(net.ParseIP(vm.IP)) {
			continue
		}

		for workloads.Contains(usedHosts, curHostID) {
			if curHostID == 254 {
				return errors.New("all 253 ips of the network are exhausted")
			}
			curHostID++
		}
		usedHosts = append(usedHosts, curHostID)
		vmIP := ip.To4()
		vmIP[3] = curHostID
		dl.Vms[idx].IP = vmIP.String()
	}
	return nil
}

func (d *DeploymentDeployer) syncContract(contractID uint64) (uint64, error) {
	sub := d.TFPluginClient.SubstrateConn

	if contractID == 0 {
		return contractID, nil
	}

	valid, err := sub.IsValidContract(contractID)
	if err != nil {
		return contractID, errors.Wrap(err, "error checking contract validity")
	}

	if !valid {
		contractID = 0
	}

	return contractID, nil
}
