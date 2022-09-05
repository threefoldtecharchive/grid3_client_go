package loader

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func LoadK8sFromGrid(manager deployer.DeploymentManager, masterNode map[uint32]string, workerNodes map[uint32][]string) (workloads.K8sCluster, error) {
	ret := workloads.K8sCluster{}
	nodes := []uint32{}

	for nodeID := range masterNode {
		nodes = append(nodes, nodeID)
	}
	for nodeID := range workerNodes {
		nodes = append(nodes, nodeID)
	}
	publicIPs := make(map[string]string)
	publicIP6s := make(map[string]string)
	diskSize := make(map[string]int)
	workloadDiskSize := make(map[string]int)
	workloadComputedIP := make(map[string]string)
	workloadComputedIP6 := make(map[string]string)
	currentDeployments := map[uint32]gridtypes.Deployment{}

	for idx := range nodes {
		dl, err := manager.GetDeployment(nodes[idx])
		if err != nil {
			return workloads.K8sCluster{}, err
		}
		currentDeployments[nodes[idx]] = dl
		for _, w := range dl.Workloads {
			if w.Type == zos.PublicIPType {
				d := zos.PublicIPResult{}
				if err := json.Unmarshal(w.Result.Data, &d); err != nil {
					log.Printf("failed to load pubip data %s", err)
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
	}

	for _, dl := range currentDeployments {
		for _, w := range dl.Workloads {
			if w.Type == zos.ZMachineType {
				publicIPKey := fmt.Sprintf("%sip", w.Name)
				diskKey := fmt.Sprintf("%sdisk", w.Name)
				workloadDiskSize[string(w.Name)] = diskSize[diskKey]
				workloadComputedIP[string(w.Name)] = publicIPs[publicIPKey]
				workloadComputedIP6[string(w.Name)] = publicIP6s[publicIPKey]
			}
		}
	}

	for nodeID, masterName := range masterNode {

		wl, err := manager.GetWorkload(nodeID, masterName)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", masterName)
		}
		dataI, err := wl.WorkloadData()
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s data", masterName)
		}
		data, ok := dataI.(*zos.ZMachine)
		if !ok {
			return workloads.K8sCluster{}, errors.New("couldn't cast workload data")
		}
		ret.NetworkName = data.Network.Interfaces[0].Network.String()
		ret.SSHKey = data.Env["SSH_KEY"]
		ret.Token = data.Env["K3S_TOKEN"]
		var result zos.ZMachineResult
		err = wl.Result.Unmarshal(&result)
		if err != nil {
			return workloads.K8sCluster{}, err
		}
		master := generateK8sNodeData(masterName, nodeID, data, workloadComputedIP[masterName], workloadComputedIP6[masterName], result.YggIP, result.IP, workloadDiskSize[masterName])
		ret.Master = &master
	}

	for nodeID, workerNames := range workerNodes {
		for _, name := range workerNames {
			wl, err := manager.GetWorkload(nodeID, name)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", name)
			}
			dataI, err := wl.WorkloadData()
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s data", name)
			}
			data, ok := dataI.(*zos.ZMachine)
			if !ok {
				return workloads.K8sCluster{}, errors.New("couldn't cast workload data")
			}
			var result zos.ZMachineResult
			err = wl.Result.Unmarshal(&result)
			if err != nil {
				return workloads.K8sCluster{}, err
			}
			worker := generateK8sNodeData(name, nodeID, data, workloadComputedIP[name], workloadComputedIP6[name], result.YggIP, result.IP, workloadDiskSize[name])
			ret.Workers = append(ret.Workers, worker)
		}
	}
	return ret, nil
}

func generateK8sNodeData(name string, nodeID uint32, data *zos.ZMachine, computedIP string, computedIP6 string, yggIP string, ip string, diskSize int) workloads.K8sNodeData {
	return workloads.K8sNodeData{
		Name:        name,
		Node:        nodeID,
		DiskSize:    diskSize,
		PublicIP:    computedIP != "",
		PublicIP6:   computedIP6 != "",
		Planetary:   yggIP != "",
		Flist:       data.FList,
		ComputedIP:  computedIP,
		ComputedIP6: computedIP6,
		YggIP:       yggIP,
		IP:          data.Network.Interfaces[0].IP.String(),
		Cpu:         int(data.ComputeCapacity.CPU),
		Memory:      int(data.ComputeCapacity.Memory / gridtypes.Megabyte),
	}
}
