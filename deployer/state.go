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

// ContractIDs represents a slice of contract IDs
type ContractIDs []uint64

// State struct
type State struct {
	CurrentNodeDeployments map[uint32]ContractIDs
	// TODO: remove it and merge with deployments
	CurrentNodeNetworks map[uint32]ContractIDs

	networks NetworkState

	ncPool    client.NodeClientGetter
	substrate subi.SubstrateExt
}

// NewState generates a new state
func NewState(ncPool client.NodeClientGetter, substrate subi.SubstrateExt) *State {
	return &State{
		CurrentNodeDeployments: make(map[uint32]ContractIDs),
		CurrentNodeNetworks:    make(map[uint32]ContractIDs),
		networks:               NetworkState{},
		ncPool:                 ncPool,
		substrate:              substrate,
	}
}

// LoadDiskFromGrid loads a disk from grid
func (st *State) LoadDiskFromGrid(nodeID uint32, name string, deploymentName string) (workloads.Disk, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewDiskFromWorkload(&wl)
}

// LoadGatewayFQDNFromGrid loads a gateway FQDN proxy from grid
func (st *State) LoadGatewayFQDNFromGrid(nodeID uint32, name string, deploymentName string) (workloads.GatewayFQDNProxy, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	deploymentData, err := workloads.ParseDeploymentData(dl.Metadata)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "could not generate deployment metadata for %s", name)
	}
	gateway, err := workloads.NewGatewayFQDNProxyFromZosWorkload(wl)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, err
	}
	gateway.ContractID = dl.ContractID
	gateway.NodeID = nodeID
	gateway.SolutionType = deploymentData.ProjectName
	gateway.NodeDeploymentID = map[uint32]uint64{nodeID: dl.ContractID}
	return gateway, nil
}

// LoadQSFSFromGrid loads a QSFS from grid
func (st *State) LoadQSFSFromGrid(nodeID uint32, name string, deploymentName string) (workloads.QSFS, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.QSFS{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewQSFSFromWorkload(&wl)
}

// LoadGatewayNameFromGrid loads a gateway name proxy from grid
func (st *State) LoadGatewayNameFromGrid(nodeID uint32, name string, deploymentName string) (workloads.GatewayNameProxy, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	nameContractID, err := st.substrate.GetContractIDByNameRegistration(wl.Name.String())
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "failed to get gateway name contract %s", name)
	}
	deploymentData, err := workloads.ParseDeploymentData(dl.Metadata)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "could not generate deployment metadata for %s", name)
	}
	gateway, err := workloads.NewGatewayNameProxyFromZosWorkload(wl)
	if err != nil {
		return workloads.GatewayNameProxy{}, err
	}
	gateway.NameContractID = nameContractID
	gateway.ContractID = dl.ContractID
	gateway.NodeID = nodeID
	gateway.SolutionType = deploymentData.ProjectName
	gateway.NodeDeploymentID = map[uint32]uint64{nodeID: dl.ContractID}
	return gateway, nil
}

// LoadZdbFromGrid loads a zdb from grid
func (st *State) LoadZdbFromGrid(nodeID uint32, name string, deploymentName string) (workloads.ZDB, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "could not get workload from node %d within deployment %v", nodeID, dl)
	}

	return workloads.NewZDBFromWorkload(&wl)
}

// LoadVMFromGrid loads a vm from a grid
func (st *State) LoadVMFromGrid(nodeID uint32, name string, deploymentName string) (workloads.VM, error) {
	wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "could not get workload from node %d", nodeID)
	}

	return workloads.NewVMFromWorkload(&wl, &dl)
}

// LoadK8sFromGrid loads k8s from grid
func (st *State) LoadK8sFromGrid(masterNode map[uint32]string, workerNodes map[uint32][]string, deploymentName string) (workloads.K8sCluster, error) {
	cluster := workloads.K8sCluster{}

	nodeDeploymentID := map[uint32]uint64{}
	for nodeID, name := range masterNode {
		wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not get workload %s", name)
		}

		workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := st.computeK8sDeploymentResources(nodeID, dl)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not compute master %s, resources", name)
		}

		master, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not generate master data for %s", name)
		}

		cluster.Master = &master
		nodeDeploymentID[nodeID] = dl.ContractID

		deploymentData, err := workloads.ParseDeploymentData(dl.Metadata)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "could not generate master deployment metadata for %s", name)
		}
		cluster.SolutionType = deploymentData.ProjectName
	}

	for nodeID, workerNames := range workerNodes {
		for _, name := range workerNames {
			wl, dl, err := st.GetWorkloadInDeployment(nodeID, name, deploymentName)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not get workload %s", name)
			}

			workloadDiskSize, workloadComputedIP, workloadComputedIP6, err := st.computeK8sDeploymentResources(nodeID, dl)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not compute worker %s, resources", name)
			}

			worker, err := workloads.NewK8sNodeFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "could not generate worker data for %s", name)
			}

			cluster.Workers = append(cluster.Workers, worker)
			nodeDeploymentID[nodeID] = dl.ContractID
		}
	}
	cluster.NodeDeploymentID = nodeDeploymentID
	return cluster, nil
}

func (st *State) computeK8sDeploymentResources(nodeID uint32, dl gridtypes.Deployment) (
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
func (st *State) LoadNetworkFromGrid(name string) (znet workloads.ZNet, err error) {
	sub := st.substrate
	for nodeID := range st.CurrentNodeNetworks {
		nodeClient, err := st.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return znet, errors.Wrapf(err, "could not get node client: %d", nodeID)
		}

		for _, contractID := range st.CurrentNodeNetworks[nodeID] {
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

// LoadDeploymentFromGrid loads deployment from grid
func (st *State) LoadDeploymentFromGrid(nodeID uint32, name string) (workloads.Deployment, error) {
	deployment, err := st.GetDeploymentByName(nodeID, name)
	if err != nil {
		return workloads.Deployment{}, err
	}
	return workloads.NewDeploymentFromZosDeployment(deployment, nodeID)
}

// GetDeploymentByName returns a deployment using its name
func (st *State) GetDeploymentByName(nodeID uint32, name string) (gridtypes.Deployment, error) {
	sub := st.substrate
	if contractIDs, ok := st.CurrentNodeDeployments[nodeID]; ok {
		nodeClient, err := st.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return gridtypes.Deployment{}, errors.Wrapf(err, "could not get node client: %d", nodeID)
		}

		for _, contractID := range contractIDs {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return gridtypes.Deployment{}, errors.Wrapf(err, "could not get deployment %d from node %d", contractID, nodeID)
			}

			dlData, err := workloads.ParseDeploymentData(dl.Metadata)
			if err != nil {
				return gridtypes.Deployment{}, errors.Wrapf(err, "could not get deployment %d data", contractID)
			}

			if dlData.Name != name {
				continue
			}
			return dl, nil
		}
		return gridtypes.Deployment{}, fmt.Errorf("could not get deployment with name %s", name)
	}
	return gridtypes.Deployment{}, fmt.Errorf("could not get deployment '%s' with node ID %d", name, nodeID)
}

// GetWorkloadInDeployment return a workload in a deployment using their names and node ID
func (st *State) GetWorkloadInDeployment(nodeID uint32, name string, deploymentName string) (gridtypes.Workload, gridtypes.Deployment, error) {
	sub := st.substrate
	if contractIDs, ok := st.CurrentNodeDeployments[nodeID]; ok {
		nodeClient, err := st.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not get node client: %d", nodeID)
		}

		for _, contractID := range contractIDs {
			dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
			if err != nil {
				return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not get deployment %d from node %d", contractID, nodeID)
			}

			dlData, err := workloads.ParseDeploymentData(dl.Metadata)
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

// GetNetworks gets state networks
func (st *State) GetNetworks() NetworkState {
	return st.networks
}

// SetNetworks sets state networks
func (st *State) SetNetworks(networks NetworkState) {
	st.networks = networks
}
