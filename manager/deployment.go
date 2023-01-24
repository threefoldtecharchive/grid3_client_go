// Package manager is grid manager
package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// DeploymentDeployer for deploying a deployment
type DeploymentDeployer struct {
	ID          string
	Node        uint32
	Disks       []workloads.Disk
	ZDBs        []workloads.ZDB
	VMs         []workloads.VM
	QSFSs       []workloads.QSFS
	IPRange     string
	NetworkName string

	TFPluginClient *TFPluginClient
	ncPool         client.NodeClientGetter
	deployer       deployer.DeployerInterface
}

// DeploymentData for deployments meta data
type DeploymentData struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

// NewDeploymentDeployer generates a new deployer for a deployment
func NewDeploymentDeployer(d *workloads.Deployment, tfPluginClient *TFPluginClient) (DeploymentDeployer, error) {
	pool := client.NewNodeClientPool(tfPluginClient.rmb)
	deploymentData := DeploymentData{
		Name:        d.Name,
		Type:        "vm",
		ProjectName: d.SolutionType,
	}
	deploymentDataBytes, err := json.Marshal(deploymentData)
	if err != nil {
		log.Printf("error parsing deployment data: %s", err.Error())
	}

	znet, err := LoadNetworkFromGrid(tfPluginClient.manager, d.NetworkName)
	if err != nil {
		log.Printf("error getting network workload: %s", err.Error())
	}
	ipRange := znet.IPRange.IP.String()

	newDeployer := deployer.NewDeployer(tfPluginClient.identity, tfPluginClient.twinID, tfPluginClient.gridProxyClient, pool, true, d.SolutionProvider, string(deploymentDataBytes))

	deploymentDeployer := DeploymentDeployer{
		ID:             "",
		Node:           d.NodeID,
		Disks:          d.Disks,
		VMs:            d.Vms,
		QSFSs:          d.Qsfs,
		ZDBs:           d.Zdbs,
		IPRange:        ipRange,
		NetworkName:    d.NetworkName,
		TFPluginClient: tfPluginClient,
		ncPool:         pool,
		deployer:       &newDeployer,
	}
	return deploymentDeployer, nil
}

func (d *DeploymentDeployer) assignNodesIPs() error {
	usedHosts := d.TFPluginClient.manager.GetUsedNetworkHostIDs(d.NetworkName, d.Node)
	if len(d.VMs) == 0 {
		return nil
	}
	ip, ipRangeCIDR, err := net.ParseCIDR(d.IPRange)
	if err != nil {
		return errors.Wrapf(err, "invalid ip %s", d.IPRange)
	}
	for _, vm := range d.VMs {
		vmIP := net.ParseIP(vm.IP)
		vmHostID := vmIP[3]
		if vm.IP != "" && ipRangeCIDR.Contains(vmIP) && !workloads.Contains(usedHosts, vmHostID) {
			usedHosts = append(usedHosts, vmHostID)
		}
	}
	curHostID := byte(2)

	for idx, vm := range d.VMs {

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
		vmIP := ip
		vmIP[3] = curHostID
		d.VMs[idx].IP = vmIP.String()
	}
	return nil
}

// GenerateVersionlessDeployments generates a new deployment without a version
func (d *DeploymentDeployer) GenerateVersionlessDeployments(ctx context.Context) (map[uint32]gridtypes.Deployment, error) {
	dl := workloads.NewDeployment(d.TFPluginClient.twinID, []gridtypes.Workload{})
	err := d.assignNodesIPs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	for _, disk := range d.Disks {
		dl.Workloads = append(dl.Workloads, disk.GenerateWorkload())
	}
	for _, zdb := range d.ZDBs {
		dl.Workloads = append(dl.Workloads, zdb.GenerateWorkload())
	}
	for _, vm := range d.VMs {
		dl.Workloads = append(dl.Workloads, vm.GenerateVMWorkload()...)
	}

	for idx, q := range d.QSFSs {
		qsfsWorkload, err := q.ZosWorkload()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate qsfs %d", idx)
		}
		dl.Workloads = append(dl.Workloads, qsfsWorkload)
	}

	return map[uint32]gridtypes.Deployment{d.Node: dl}, nil
}

// GetOldDeployments returns old deployments IDs with their nodes IDs
func (d *DeploymentDeployer) GetOldDeployments(ctx context.Context) (map[uint32]uint64, error) {
	deployments := make(map[uint32]uint64)
	if d.ID != "" {

		deploymentID, err := strconv.ParseUint(d.ID, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't parse deployment id %s", d.ID)
		}
		deployments[d.Node] = deploymentID
	}

	return deployments, nil
}

// Nullify resets deployments
func (d *DeploymentDeployer) Nullify() {
	d.VMs = nil
	d.QSFSs = nil
	d.Disks = nil
	d.ZDBs = nil
	d.ID = ""
}

func (d *DeploymentDeployer) parseID() uint64 {
	id, err := strconv.ParseUint(d.ID, 10, 64)
	if err != nil {
		panic(err)
	}
	return id

}
func (d *DeploymentDeployer) syncContract(sub subi.SubstrateExt) error {
	if d.ID == "" {
		return nil
	}
	valid, err := sub.IsValidContract(d.parseID())
	if err != nil {
		return errors.Wrap(err, "error checking contract validity")
	}
	if !valid {
		d.ID = ""
		return nil
	}
	return nil
}

// Sync syncs the deployments
func (d *DeploymentDeployer) Sync(ctx context.Context, sub subi.SubstrateExt, cl *TFPluginClient) error {
	if err := d.syncContract(sub); err != nil {
		return err
	}
	if d.ID == "" {
		d.Nullify()
		return nil
	}
	currentDeployments, err := d.deployer.GetDeployments(ctx, sub, map[uint32]uint64{d.Node: d.parseID()})
	if err != nil {
		return errors.Wrap(err, "failed to get deployments to update local state")
	}
	dl := currentDeployments[d.Node]
	var vms []workloads.VM
	var zdbs []workloads.ZDB
	var qsfs []workloads.QSFS
	var disks []workloads.Disk

	d.TFPluginClient.manager.DeleteDeploymentNetworkHostIDs(d.NetworkName, d.Node, d.ID)

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

	d.TFPluginClient.manager.SetDeploymentNetworkHostIDs(d.NetworkName, d.Node, d.ID, usedIPs)
	d.Match(disks, qsfs, zdbs, vms)
	log.Printf("vms: %+v\n", len(vms))
	d.Disks = disks
	d.QSFSs = qsfs
	d.ZDBs = zdbs
	d.VMs = vms
	return nil
}

// Match objects to match the input
//
//	already existing object are stored ordered the same way they are in the input
//	others are pushed after
func (d *DeploymentDeployer) Match(
	disks []workloads.Disk,
	qsfs []workloads.QSFS,
	zdbs []workloads.ZDB,
	vms []workloads.VM,
) {
	vmMap := make(map[string]*workloads.VM)
	l := len(d.Disks) + len(d.QSFSs) + len(d.ZDBs) + len(d.VMs)
	names := make(map[string]int)
	for idx, o := range d.Disks {
		names[o.Name] = idx - l
	}
	for idx, o := range d.QSFSs {
		names[o.Name] = idx - l
	}
	for idx, o := range d.ZDBs {
		names[o.Name] = idx - l
	}
	for idx, o := range d.VMs {
		names[o.Name] = idx - l
		vmMap[o.Name] = &d.VMs[idx]
	}
	sort.Slice(disks, func(i, j int) bool {
		return names[disks[i].Name] < names[disks[j].Name]
	})
	sort.Slice(qsfs, func(i, j int) bool {
		return names[qsfs[i].Name] < names[qsfs[j].Name]
	})
	sort.Slice(zdbs, func(i, j int) bool {
		return names[zdbs[i].Name] < names[zdbs[j].Name]
	})
	sort.Slice(vms, func(i, j int) bool {
		return names[vms[i].Name] < names[vms[j].Name]
	})
	for idx := range vms {
		vm, ok := vmMap[vms[idx].Name]
		if ok {
			vms[idx].Match(vm)
			log.Printf("orig: %+v\n", vm)
			log.Printf("new: %+v\n", vms[idx])
		}
	}
}

// TODO: are there any more validations on workloads needed other than vm and network name relation?
func (d *DeploymentDeployer) validate() error {
	if len(d.VMs) != 0 && len(strings.TrimSpace(d.NetworkName)) == 0 {
		return errors.New("if you pass a vm, network_name must be non-empty")
	}

	for _, vm := range d.VMs {
		if err := vm.Validate(); err != nil {
			return errors.Wrapf(err, "vm %s validation failed", vm.Name)
		}
	}
	return nil
}

// Deploy deploys a new deployment
func (d *DeploymentDeployer) Deploy(ctx context.Context, sub subi.SubstrateExt) error {
	if err := d.validate(); err != nil {
		return err
	}
	newDeployments, err := d.GenerateVersionlessDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}
	oldDeployments, err := d.GetOldDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't get old deployments data")
	}
	currentDeployments, err := d.deployer.Deploy(ctx, sub, oldDeployments, newDeployments)
	if currentDeployments[d.Node] != 0 {
		d.ID = fmt.Sprintf("%d", currentDeployments[d.Node])
	}
	return err
}

// Cancel cancels deployments
func (d *DeploymentDeployer) Cancel(ctx context.Context, sub subi.SubstrateExt) error {
	newDeployments := make(map[uint32]gridtypes.Deployment)
	oldDeployments, err := d.GetOldDeployments(ctx)
	if err != nil {
		return err
	}
	currentDeployments, err := d.deployer.Deploy(ctx, sub, oldDeployments, newDeployments)
	id := currentDeployments[d.Node]
	if id != 0 {
		d.ID = fmt.Sprintf("%d", id)
	} else {
		d.ID = ""
	}
	return err
}
