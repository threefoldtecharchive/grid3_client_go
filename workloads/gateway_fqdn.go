package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type GatewayFQDNProxy struct {
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string

	// Passthrough whether to pass tls traffic or not
	TLSPassthrough bool

	// Backends are list of backend ips
	Backends []zos.Backend

	// FQDN deployed on the node
	FQDN string
}

func (g *GatewayFQDNProxy) GenerateWorkloadFromFQDN(gatewayFQDN GatewayFQDNProxy) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayFQDNProxyType,
		Name:    gridtypes.Name(gatewayFQDN.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			TLSPassthrough: gatewayFQDN.TLSPassthrough,
			Backends:       gatewayFQDN.Backends,
			FQDN:           gatewayFQDN.FQDN,
		}),
	}, nil
}

func (g *GatewayFQDNProxy) Stage(manager deployer.DeploymentManager, NodeId uint32) error { //ZosWorkload()
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := g.GenerateWorkloadFromFQDN(*g)
	if err != nil {
		return err
	}
	workloads = append(workloads, workload)
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}
