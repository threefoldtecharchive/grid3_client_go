// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// GatewayWorkload for tests
var GatewayNameWorkload = GatewayNameProxy{
	Name:           "test",
	TLSPassthrough: false,
	Backends:       []zos.Backend{zos.Backend("http://1.1.1.1")},
}

func TestGatewayNameProxyWorkload(t *testing.T) {
	var gateway gridtypes.Workload

	t.Run("test_gateway_from_zos_workload", func(t *testing.T) {
		gateway = GatewayNameWorkload.ZosWorkload()

		res, err := json.Marshal(zos.GatewayNameProxy{})
		assert.NoError(t, err)
		gateway.Result.Data = res

		gatewayFromWorkload, err := NewGatewayNameProxyFromZosWorkload(gateway)
		assert.NoError(t, err)

		assert.Equal(t, gatewayFromWorkload, GatewayNameWorkload)
	})

	t.Run("failed to get workload data", func(t *testing.T) {
		gatewayCp := gateway
		gatewayCp.Data = nil
		_, err := NewGatewayNameProxyFromZosWorkload(gatewayCp)
		assert.Contains(t, err.Error(), "failed to get workload data")
	})

	t.Run("test_workload_from_gateway_name", func(t *testing.T) {
		gateway.Result = gridtypes.Result{}

		workloadFromName := GatewayNameWorkload.ZosWorkload()
		assert.Equal(t, workloadFromName, gateway)
	})

	t.Run("failed to get workload result data", func(t *testing.T) {
		_, err := NewGatewayNameProxyFromZosWorkload(gateway)
		assert.Contains(t, err.Error(), "error unmarshalling json")
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], gateway)

		workloadsMap2, err := GatewayNameWorkload.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
