// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package todo

import (
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
)

type APIClient struct {
	SubstrateExt subi.SubstrateExt
	NCPool       *client.NodeClientPool
	ProxyClient  proxy.Client
	Manager      deployer.DeploymentManager
	Identity     subi.Identity
}
