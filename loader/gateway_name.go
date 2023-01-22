// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadGatewayNameFromGrid loads a gateway name proxy from grid
func LoadGatewayNameFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.GatewayNameProxy, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewGatewayNameProxyFromZosWorkload(wl)
}
