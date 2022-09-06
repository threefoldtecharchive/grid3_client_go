package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
)

type Workload interface {
	Stage(d deployer.DeploymentManager, NodeID uint32) error
}

type APIClient struct {
	SubstrateExt subi.SubstrateExt
	NCPool       *client.NodeClientPool
	ProxyClient  proxy.Client
	Manager      deployer.DeploymentManager
	Identity     subi.Identity
}
