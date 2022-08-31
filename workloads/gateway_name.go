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

func (g *GatewayNameProxy) GenerateWorkloadFromGName(gatewayName GatewayNameProxy) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(gatewayName.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           gatewayName.Name,
			TLSPassthrough: gatewayName.TLSPassthrough,
			Backends:       gatewayName.Backends,
		}),
	}, nil
}

func (g *GatewayNameProxy) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := g.GenerateWorkloadFromGName(*g)
	if err != nil {
		return err
	}
	workloads = append(workloads, workload)
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}
