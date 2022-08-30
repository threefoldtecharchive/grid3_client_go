package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
)

type Workload interface {
	Stage(d deployer.DeploymentManager, NodeID uint32) error
}
