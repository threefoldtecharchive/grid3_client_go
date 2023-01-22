// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	deployer "github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadVMFromGrid loads a vm from a grid
func LoadVMFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.VM, error) {
	dl, err := manager.GetDeployment(nodeID)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to get deployment with id %d", nodeID)
	}

	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewVMFromWorkloads(wl, dl)
}
