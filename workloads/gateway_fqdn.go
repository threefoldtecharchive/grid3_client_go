// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// GatewayFQDNProxy for gateway FQDN proxy
type GatewayFQDNProxy struct {
	NodeID uint32
	// Backends are list of backend ips
	Backends []zos.Backend
	// FQDN deployed on the node
	FQDN string

	// optional
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string
	// Passthrough whether to pass tls traffic or not
	TLSPassthrough   bool
	Description      string
	NodeDeploymentID map[uint32]uint64
	SolutionType     string

	// computed
	ContractID uint64
}

// NewGatewayFQDNProxyFromZosWorkload generates a gateway FQDN proxy from a zos workload
func NewGatewayFQDNProxyFromZosWorkload(wl gridtypes.Workload) (GatewayFQDNProxy, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return GatewayFQDNProxy{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.GatewayFQDNProxy)
	if !ok {
		return GatewayFQDNProxy{}, errors.New("could not create gateway fqdn proxy workload")
	}

	return GatewayFQDNProxy{
		Name:           wl.Name.String(),
		TLSPassthrough: data.TLSPassthrough,
		Backends:       data.Backends,
		FQDN:           data.FQDN,
	}, nil
}

// ZosWorkload generates a zos workload from GatewayFQDNProxy
func (g *GatewayFQDNProxy) ZosWorkload() gridtypes.Workload {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayFQDNProxyType,
		Name:    gridtypes.Name(g.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			TLSPassthrough: g.TLSPassthrough,
			Backends:       g.Backends,
			FQDN:           g.FQDN,
		}),
	}
}

// BindWorkloadsToNode for staging workloads to node IDs
func (g *GatewayFQDNProxy) BindWorkloadsToNode(nodeID uint32) (map[uint32][]gridtypes.Workload, error) {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloadsMap[nodeID] = append(workloadsMap[nodeID], g.ZosWorkload())
	return workloadsMap, nil
}
