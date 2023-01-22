// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// GatewayFQDNProxy for gateway FQDN proxy
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

// GatewayFQDNProxyFromZosWorkload generates a gateway FQDN proxy from a zos workload
func GatewayFQDNProxyFromZosWorkload(wl gridtypes.Workload) (GatewayFQDNProxy, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return GatewayFQDNProxy{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.GatewayFQDNProxy)
	if !ok {
		return GatewayFQDNProxy{}, errors.New("couldn't cast workload data")
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

// GenerateWorkloads generates a workload from a fqdn
func (g *GatewayFQDNProxy) GenerateWorkloads() ([]gridtypes.Workload, error) {
	return []gridtypes.Workload{
		{
			Version: 0,
			Type:    zos.GatewayFQDNProxyType,
			Name:    gridtypes.Name(g.Name),
			// REVISE: whether description should be set here
			Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
				TLSPassthrough: g.TLSPassthrough,
				Backends:       g.Backends,
				FQDN:           g.FQDN,
			}),
		},
	}, nil
}

// BindWorkloadsToNode for staging workloads to node IDs
func (g *GatewayFQDNProxy) BindWorkloadsToNode(nodeID uint32) (map[uint32][]gridtypes.Workload, error) {
	workloadsMap := map[uint32][]gridtypes.Workload{}

	workloads, err := g.GenerateWorkloads()
	if err != nil {
		return workloadsMap, err
	}

	workloadsMap[nodeID] = workloads
	return workloadsMap, nil
}
