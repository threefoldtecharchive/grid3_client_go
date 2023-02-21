// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type contractIDs []uint64

// State struct
type State struct {
	currentNodeDeployments map[uint32]contractIDs
	// TODO: remove it and merge with deployments
	currentNodeNetworks map[uint32]contractIDs

	networks networkState

	ncPool    client.NodeClientGetter
	substrate subi.SubstrateExt
}

// NewState generates a new state
func NewState(ncPool client.NodeClientGetter, substrate subi.SubstrateExt) *State {
	return &State{
		currentNodeDeployments: make(map[uint32]contractIDs),
		currentNodeNetworks:    make(map[uint32]contractIDs),
		networks:               networkState{},
		ncPool:                 ncPool,
		substrate:              substrate,
	}
}

// LoadDiskFromGrid loads a disk from grid
func (l *State) LoadDiskFromGrid(nodeID uint32, name string, deploymentName string) (workloads.Disk, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewDiskFromWorkload(&wl)
}

// LoadGatewayFQDNFromGrid loads a gateway FQDN proxy from grid
func (l *State) LoadGatewayFQDNFromGrid(nodeID uint32, name string, deploymentName string) (workloads.GatewayFQDNProxy, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewGatewayFQDNProxyFromZosWorkload(wl)
}

// LoadQSFSFromGrid loads a QSFS from grid
func (l *State) LoadQSFSFromGrid(nodeID uint32, name string, deploymentName string) (workloads.QSFS, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.QSFS{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewQSFSFromWorkload(&wl)
}

// LoadGatewayNameFromGrid loads a gateway name proxy from grid
func (l *State) LoadGatewayNameFromGrid(nodeID uint32, name string, deploymentName string) (workloads.GatewayNameProxy, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewGatewayNameProxyFromZosWorkload(wl)
}

// LoadZdbFromGrid loads a zdb from grid
func (l *State) LoadZdbFromGrid(nodeID uint32, name string, deploymentName string) (workloads.ZDB, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewZDBFromWorkload(&wl)
}

// LoadVMFromGrid loads a vm from a grid
func (l *State) LoadVMFromGrid(nodeID uint32, name string, deploymentName string) (workloads.VM, error) {
	wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "could not get workload from node %d", nodeID)
	}

	return workloads.NewVMFromWorkload(&wl, &dl)
}

// LoadK8sFromGrid loads k8s from grid
func (l *State) LoadK8sFromGrid(masterNode map[uint32]string, workerNodes map[uint32][]string, deploymentName string) (workloads.K8sCluster, error) {
	cluster := workloads.K8sCluster{}

	for nodeID, name := range masterNode {
		wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not get workload %s", name)
		}

		workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := l.computeK8sDeploymentResources(nodeID, dl)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not compute master %s, resources", name)
		}

		master, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not generate master data for %s", name)
		}

		cluster.Master = &master
	}

	for nodeID, workerNames := range workerNodes {
		for _, name := range workerNames {
			wl, dl, err := l.GetWorkloadInDeployment(nodeID, name, deploymentName)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not get workload %s", name)
			}

			workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := l.computeK8sDeploymentResources(nodeID, dl)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not compute worker %s, resources", name)
			}

			worker, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not generate worker data for %s", name)
			}

			cluster.Workers = append(cluster.Workers, worker)
		}
	}
	return cluster, nil
}

func (l *State) computeK8sDeploymentResources(nodeID uint32, dl gridtypes.Deployment) (
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
		switch w.Type {
		case zos.PublicIPType:

			d := zos.PublicIPResult{}
			if err := json.Unmarshal(w.Result.Data, &d); err != nil {
				return workloadDiskSize, workloadComputedIP, workloadComputedIP6, errors.Wrap(err, "failed to load public ip data")
			}
			publicIPs[string(w.Name)] = d.IP.String()
			publicIP6s[string(w.Name)] = d.IPv6.String()

		case zos.ZMountType:

			d, err := w.WorkloadData()
			if err != nil {
				return workloadDiskSize, workloadComputedIP, workloadComputedIP6, errors.Wrap(err, "failed to load disk data")
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
func (l *State) LoadNetworkFromGrid(name string) (znet workloads.ZNet, err error) {
	sub := l.substrate
	for nodeID := range l.currentNodeNetworks {
		nodeClient, err := l.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return znet, errors.Wrapf(err, "could not get node client: %d", nodeID)
		}

		for _, contractID := range l.currentNodeNetworks[nodeID] {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return znet, errors.Wrapf(err, "could not get network deployment %d from node %d", contractID, nodeID)
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

// GetWorkloadInDeployment return a workload in a deployment using their names and node ID
func (l *State) GetWorkloadInDeployment(nodeID uint32, name string, deploymentName string) (gridtypes.Workload, gridtypes.Deployment, error) {
	sub := l.substrate
	if contractIDs, ok := l.currentNodeDeployments[nodeID]; ok {
		nodeClient, err := l.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not get node client: %d", nodeID)
		}

		for _, contractID := range contractIDs {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not get deployment %d from node %d", contractID, nodeID)
			}

			dlData, err := workloads.ParseDeploymentDate(dl.Metadata)
			if err != nil {
				return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not get deployment %d data", contractID)
			}

			if dlData.Name != deploymentName {
				continue
			}

			for _, workload := range dl.Workloads {
				if workload.Name == gridtypes.Name(name) {
					return workload, dl, nil
				}
			}
		}
		return gridtypes.Workload{}, gridtypes.Deployment{}, fmt.Errorf("could not get workload with name %s", name)
	}
	return gridtypes.Workload{}, gridtypes.Deployment{}, fmt.Errorf("could not get workload '%s' with node ID %d", name, nodeID)
}
