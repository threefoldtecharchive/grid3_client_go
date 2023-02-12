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
	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// NewDeploymentDeployer generates a new deployer for a deployment
func NewDeploymentDeployer(tfPluginClient *TFPluginClient) DeploymentDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	return DeploymentDeployer{
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}
}

// GenerateVersionlessDeployments generates a new deployment without a version
func (d *DeploymentDeployer) GenerateVersionlessDeployments(ctx context.Context, dl *workloads.Deployment) (map[uint32]gridtypes.Deployment, error) {
	newDl := workloads.NewGridDeployment(d.tfPluginClient.twinID, []gridtypes.Workload{})
	err := d.assignNodesIPs(dl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	for _, disk := range dl.Disks {
		newDl.Workloads = append(newDl.Workloads, disk.ZosWorkload())
	}
	for _, zdb := range dl.Zdbs {
		newDl.Workloads = append(newDl.Workloads, zdb.ZosWorkload())
	}
	for _, vm := range dl.Vms {
		newDl.Workloads = append(newDl.Workloads, vm.ZosWorkload()...)
	}

	for idx, q := range dl.Qsfs {
		qsfsWorkload, err := q.ZosWorkload()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate qsfs %d", idx)
		}
		newDl.Workloads = append(newDl.Workloads, qsfsWorkload)
	}

	newDl.Metadata, err = dl.GenerateMetadata()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate deployment %s metadata", dl.Name)
	}

	return map[uint32]gridtypes.Deployment{dl.NodeID: newDl}, nil
}

// Deploy deploys a new deployment
func (d *DeploymentDeployer) Deploy(ctx context.Context, dl *workloads.Deployment) error {
	if err := d.Validate(ctx, dl); err != nil {
		return err
	}

	// solution providers
	newDeploymentsSolutionProvider := map[uint32]*uint64{dl.NodeID: dl.SolutionProvider}

	newDeployments, err := d.GenerateVersionlessDeployments(ctx, dl)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment

	currentDeployments, err := d.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsSolutionProvider)

	// update deployment and plugin state
	if currentDeployments[dl.NodeID] != 0 {
		if dl.NodeDeploymentID == nil {
			dl.NodeDeploymentID = make(map[uint32]uint64)
		}

		dl.ContractID = currentDeployments[dl.NodeID]
		dl.NodeDeploymentID[dl.NodeID] = dl.ContractID
		d.tfPluginClient.StateLoader.currentNodeDeployment[dl.NodeID] = dl.ContractID
	}

	return err
}

// Cancel cancels deployments
func (d *DeploymentDeployer) Cancel(ctx context.Context, dl *workloads.Deployment) error {
	if err := d.Validate(ctx, dl); err != nil {
		return err
	}

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment

	err := d.deployer.Cancel(ctx, oldDeployments[dl.NodeID])
	if err != nil {
		return err
	}

	// update state
	dl.ContractID = 0
	delete(dl.NodeDeploymentID, dl.NodeID)
	delete(d.tfPluginClient.StateLoader.currentNodeDeployment, dl.NodeID)

	return nil
}

// Sync syncs the deployments
func (d *DeploymentDeployer) Sync(ctx context.Context, dl *workloads.Deployment) error {
	err := d.syncContract(ctx, dl)
	if err != nil {
		return err
	}
	currentDeployments, err := d.deployer.GetDeployments(ctx, d.tfPluginClient.StateLoader.currentNodeDeployment)
	if err != nil {
		return errors.Wrap(err, "failed to get deployments to update local state")
	}

	deployment := currentDeployments[dl.NodeID]

	if dl.ContractID == 0 {
		dl.Nullify()
		return nil
	}

	vms := make([]workloads.VM, 0)
	zdbs := make([]workloads.ZDB, 0)
	qsfs := make([]workloads.QSFS, 0)
	disks := make([]workloads.Disk, 0)

	network := d.tfPluginClient.StateLoader.networks.getNetwork(dl.NetworkName)
	network.deleteDeploymentHostIDs(dl.NodeID, dl.ContractID)

	usedIPs := []byte{}
	for _, w := range deployment.Workloads {
		if !w.Result.State.IsOkay() {
			continue
		}

		switch w.Type {
		case zos.ZMachineType:
			vm, err := workloads.NewVMFromWorkload(&w, &deployment)
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

	network = d.tfPluginClient.StateLoader.networks.getNetwork(dl.NetworkName)
	network.setDeploymentHostIDs(dl.NodeID, dl.ContractID, usedIPs)

	dl.Match(disks, qsfs, zdbs, vms)

	dl.Disks = disks
	dl.Qsfs = qsfs
	dl.Zdbs = zdbs
	dl.Vms = vms

	return nil
}

// Validate validates a deployment deployer
func (d *DeploymentDeployer) Validate(ctx context.Context, dl *workloads.Deployment) error {
	sub := d.tfPluginClient.SubstrateConn

	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.identity); err != nil {
		return err
	}

	return dl.Validate()
}

func (d *DeploymentDeployer) assignNodesIPs(dl *workloads.Deployment) error {
	network := d.tfPluginClient.StateLoader.networks.getNetwork(dl.NetworkName)
	ipRange := network.getNodeSubnet(dl.NodeID)

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
			if ipRangeCIDR.Contains(vmIP) && !workloads.Contains(usedHosts, vmHostID) {
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

func (d *DeploymentDeployer) syncContract(ctx context.Context, dl *workloads.Deployment) error {
	sub := d.tfPluginClient.SubstrateConn

	if dl.ContractID == 0 {
		return nil
	}

	valid, err := sub.IsValidContract(dl.ContractID)
	if err != nil {
		return errors.Wrap(err, "error checking contract validity")
	}

	if !valid {
		dl.ContractID = 0
	}

	return nil
}
