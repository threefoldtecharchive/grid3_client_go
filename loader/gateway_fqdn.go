// Package loader to load different types, workloads from grid
package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

// LoadGatewayFqdnFromGrid loads a gateway FQDN proxy from grid
func LoadGatewayFqdnFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.GatewayFQDNProxy, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	return workloads.NewGatewayFQDNProxyFromZosWorkload(wl)
}
