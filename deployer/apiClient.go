package deployer

import (
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
)

// APIClient struct
type APIClient struct {
	SubstrateExt subi.SubstrateExt
	NCPool       *client.NodeClientPool
	ProxyClient  proxy.Client
	Manager      DeploymentManager
	Identity     subi.Identity
}
