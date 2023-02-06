// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// GatewayWorkload for tests
var GatewayFQDNWorkload = GatewayFQDNProxy{
	Name:           "test",
	TLSPassthrough: false,
	Backends:       []zos.Backend{zos.Backend("http://1.1.1.1")},
	FQDN:           "test",
}

func TestGatewayFQDNProxyWorkload(t *testing.T) {
	var gateway gridtypes.Workload

	t.Run("test_gateway_from_zos_workload", func(t *testing.T) {
		gateway = GatewayFQDNWorkload.ZosWorkload()

		gatewayFromWorkload, err := NewGatewayFQDNProxyFromZosWorkload(gateway)
		assert.NoError(t, err)

		assert.Equal(t, gatewayFromWorkload, GatewayFQDNWorkload)
	})

	t.Run("failed to get workload data", func(t *testing.T) {
		gatewayCp := gateway
		gatewayCp.Data = nil
		_, err := NewGatewayFQDNProxyFromZosWorkload(gatewayCp)
		assert.Contains(t, err.Error(), "failed to get workload data")
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], gateway)

		workloadsMap2, err := GatewayFQDNWorkload.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
