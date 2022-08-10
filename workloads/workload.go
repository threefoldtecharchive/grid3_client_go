package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
)

type Workload interface {
	Convert(d deployer.DeploymentManager)
}
