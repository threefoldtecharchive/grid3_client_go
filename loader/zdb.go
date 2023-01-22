// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadZdbFromGrid loads a zdb from grid
func LoadZdbFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.ZDB, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewZDBFromWorkload(wl)
}
