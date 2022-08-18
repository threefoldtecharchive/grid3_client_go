package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type GatewayNameProxy struct {
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string

	// Passthrough whether to pass tls traffic or not
	TLSPassthrough bool

	// Backends are list of backend ips
	Backends []zos.Backend

	// FQDN deployed on the node
	FQDN string
}

func (g *GatewayNameProxy) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloads := make([]gridtypes.Workload, 0)
	workload := gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(g.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           g.Name,
			TLSPassthrough: g.TLSPassthrough,
			Backends:       g.Backends,
		}),
	}
	workloads = append(workloads, workload)
	err := manager.SetWorkloads(NodeId, workloads)
	return err
}
