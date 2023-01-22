// Package loader to load different types, workloads from grid
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

// LoadK8sFromGrid loads k8s from grid
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

	for nodeID, name := range masterNode {
		wl, err := manager.GetWorkload(nodeID, name)
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", name)
		}

		master, err := workloads.NewK8sNodeDataFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
		if err != nil {
			return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't generate master data for %s", name)
		}

		ret.Master = &master

	}

	for nodeID, workerNames := range workerNodes {
		for _, name := range workerNames {
			wl, err := manager.GetWorkload(nodeID, name)
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't get workload %s", name)
			}

			worker, err := workloads.NewK8sNodeDataFromWorkload(wl, nodeID, workloadDiskSize[name], workloadComputedIP[name], workloadComputedIP6[name])
			if err != nil {
				return workloads.K8sCluster{}, errors.Wrapf(err, "couldn't generate worker data for %s", name)
			}

			ret.Workers = append(ret.Workers, worker)
		}
	}
	return ret, nil
}
