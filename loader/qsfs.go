// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadQsfsFromGrid loads a qsfs from grid
func LoadQsfsFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.QSFS, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.QSFS{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewQSFSFromWorkload(wl)
}
