// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadDiskFromGrid loads a disk from grid
func LoadDiskFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.Disk, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewDiskFromWorkload(wl)
}
