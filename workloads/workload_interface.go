// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import "github.com/threefoldtech/grid3-go/deployer"

// Workload interface for workloads
type Workload interface {
	Stage(d deployer.DeploymentManager, NodeID uint32) error
}
