// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// StateLoader struct
type StateLoader struct {
	currentNodeDeployment map[uint32][]uint64
	currentNodeNetwork    map[uint32][]uint64

	networks networkState

	ncPool    client.NodeClientGetter
	substrate subi.SubstrateExt
}

// NewStateLoader generates a new loader
func NewStateLoader(ncPool client.NodeClientGetter, substrate subi.SubstrateExt) *StateLoader {
	return &StateLoader{
		currentNodeDeployment: make(map[uint32][]uint64),
		currentNodeNetwork:    make(map[uint32][]uint64),
		networks:              networkState{},
		ncPool:                ncPool,
		substrate:             substrate,
	}
}

// LoadDiskFromGrid loads a disk from grid
func (l *StateLoader) LoadDiskFromGrid(nodeID uint32, name string) (workloads.Disk, error) {
	wl, _, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewDiskFromWorkload(&wl)
}

// LoadGatewayFqdnFromGrid loads a gateway FQDN proxy from grid
func (l *StateLoader) LoadGatewayFqdnFromGrid(nodeID uint32, name string) (workloads.GatewayFQDNProxy, error) {
	wl, _, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewGatewayFQDNProxyFromZosWorkload(wl)
}

// LoadQsfsFromGrid loads a qsfs from grid
func (l *StateLoader) LoadQsfsFromGrid(nodeID uint32, name string) (workloads.QSFS, error) {
	wl, _, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.QSFS{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewQSFSFromWorkload(&wl)
}

// LoadGatewayNameFromGrid loads a gateway name proxy from grid
func (l *StateLoader) LoadGatewayNameFromGrid(nodeID uint32, name string) (workloads.GatewayNameProxy, error) {
	wl, _, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewGatewayNameProxyFromZosWorkload(wl)
}

// LoadZdbFromGrid loads a zdb from grid
func (l *StateLoader) LoadZdbFromGrid(nodeID uint32, name string) (workloads.ZDB, error) {
	wl, _, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewZDBFromWorkload(&wl)
}

// LoadVMFromGrid loads a vm from a grid
func (l *StateLoader) LoadVMFromGrid(nodeID uint32, name string) (workloads.VM, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewVMFromWorkload(&wl, &dl)
}

// LoadK8sFromGrid loads k8s from grid
func (l *StateLoader) LoadK8sFromGrid(masterNode map[uint32]string, workerNodes map[uint32][]string) (workloads.K8sCluster, error) {
	ret := workloads.K8sCluster{}

	for nodeID, name := range masterNode {
		wl, dl, err := l.GetWorkloadInDeployment(nodeID, name)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", name)
		}

		workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := l.computeK8sDeploymentResources(nodeID, dl)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't compute master %s, resources", name)
		}

		master, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't generate master data for %s", name)
		}

		ret.Master = &master
	}

	for nodeID, workerNames := range workerNodes {
		for _, name := range workerNames {
			wl, dl, err := l.GetWorkloadInDeployment(nodeID, name)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", name)
			}

			workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := l.computeK8sDeploymentResources(nodeID, dl)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't compute worker %s, resources", name)
			}

			worker, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't generate worker data for %s", name)
			}

			ret.Workers = append(ret.Workers, worker)
		}
	}
	return ret, nil
}

func (l *StateLoader) computeK8sDeploymentResources(nodeID uint32, dl gridtypes.Deployment) (
	workloadDiskSize map[string]int,
	workloadComputedIP map[string]string,
	workloadComputedIP6 map[string]string,
	err error,
) {
	workloadDiskSize = make(map[string]int)
	workloadComputedIP = make(map[string]string)
	workloadComputedIP6 = make(map[string]string)

	publicIPs := make(map[string]string)
	publicIP6s := make(map[string]string)
	diskSize := make(map[string]int)

	for _, w := range dl.Workloads {
		if w.Type == zos.PublicIPType {
			d := zos.PublicIPResult{}
			if err := json.Unmarshal(w.Result.Data, &d); err != nil {
				log.Printf("failed to load public ip data %s", err)
				continue
			}
			publicIPs[string(w.Name)] = d.IP.String()
			publicIP6s[string(w.Name)] = d.IPv6.String()
		} else if w.Type == zos.ZMountType {
			d, err := w.WorkloadData()
			if err != nil {
				log.Printf("failed to load disk data %s", err)
				continue
			}
			diskSize[string(w.Name)] = int(d.(*zos.ZMount).Size / gridtypes.Gigabyte)
		}
	}

	for _, w := range dl.Workloads {
		if w.Type == zos.ZMachineType {
			publicIPKey := fmt.Sprintf("%sip", w.Name)
			diskKey := fmt.Sprintf("%sdisk", w.Name)
			workloadDiskSize[string(w.Name)] = diskSize[diskKey]
			workloadComputedIP[string(w.Name)] = publicIPs[publicIPKey]
			workloadComputedIP6[string(w.Name)] = publicIP6s[publicIPKey]
		}
	}

	return
}

// LoadNetworkFromGrid loads a network from grid
func (l *StateLoader) LoadNetworkFromGrid(name string) (znet workloads.ZNet, err error) {
	sub := l.substrate
	for nodeID := range l.currentNodeNetwork {
		nodeClient, err := l.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return znet, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}

		for _, contractID := range l.currentNodeNetwork[nodeID] {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return znet, errors.Wrapf(err, "couldn't get network deployment %d from node %d", contractID, nodeID)
			}

			for _, wl := range dl.Workloads {
				if wl.Type == zos.NetworkType && wl.Name == gridtypes.Name(name) {
					znet, err = workloads.NewNetworkFromWorkload(wl, nodeID)
					if err != nil {
						return workloads.ZNet{}, errors.Wrapf(err, "failed to get network from workload %s", name)
					}
					break
				}
			}
		}
	}

	if reflect.DeepEqual(znet, workloads.ZNet{}) {
		return znet, errors.Errorf("failed to get network %s", name)
	}

	return znet, nil
}

// GetWorkloadInDeployment returns a workload of a deployment on a given node using its name
// TODO: make sure of unique workloads names
func (l *StateLoader) GetWorkloadInDeployment(nodeID uint32, name string) (gridtypes.Workload, gridtypes.Deployment, error) {
	sub := l.substrate
	if contractIDs, ok := l.currentNodeDeployment[nodeID]; ok {
		nodeClient, err := l.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}

		for _, contractID := range contractIDs {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "couldn't get deployment %d from node %d", contractID, nodeID)
			}

			for _, workload := range dl.Workloads {
				if workload.Name == gridtypes.Name(name) {
					return workload, dl, nil
				}
			}
		}
		return gridtypes.Workload{}, gridtypes.Deployment{}, fmt.Errorf("couldn't get workload with name %s", name)
	}
	return gridtypes.Workload{}, gridtypes.Deployment{}, fmt.Errorf("couldn't get workload '%s' with node ID %d", name, nodeID)
}
