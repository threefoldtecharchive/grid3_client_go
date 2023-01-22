// Package loader to load different types, workloads from grid
package loader

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// LoadNetworkFromGrid loads a network from grid
func LoadNetworkFromGrid(manager deployer.DeploymentManager, name string) (workloads.ZNet, error) {
	znet := workloads.ZNet{}

	for nodeID, contractID := range manager.GetContractIDs() {
		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return znet, errors.Wrapf(err, "failed to get deployment with id %d", nodeID)
		}

		for _, wl := range dl.Workloads {
			if wl.Type == zos.NetworkType && wl.Name == gridtypes.Name(name) {
				znet, err = workloads.NewNetworkFromWorkload(wl, nodeID, contractID)
				if err != nil {
					return workloads.ZNet{}, errors.Wrapf(err, "failed to get network from workload %s", name)
				}
				break
			}
		}
	}

	if reflect.DeepEqual(znet, workloads.ZNet{}) {
		return znet, errors.Errorf("failed to get network %s", name)
	}

	return znet, nil
}
