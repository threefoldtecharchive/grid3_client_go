// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayNameProxyWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gatewayName := "test"
	gatewayZosBackend := zos.Backend("http://1.1.1.1")

	var gateway GatewayNameProxy

	res, err := json.Marshal(zos.GatewayNameProxy{
		Name: "test",
	})
	assert.NoError(t, err)

	gatewayWorkload := gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(gatewayName),
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           gatewayName,
			TLSPassthrough: true,
			Backends:       []zos.Backend{gatewayZosBackend},
		}),
		Result: gridtypes.Result{
			Created: 5000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
	}

	t.Run("test_gateway_from_zos_workload", func(t *testing.T) {
		var err error

		gateway, err = NewGatewayNameProxyFromZosWorkload(gatewayWorkload)
		assert.NoError(t, err)
	})

	t.Run("test_gateway_functions", func(t *testing.T) {
		gatewayWorkloadCp := gatewayWorkload
		gatewayWorkloadCp.Result = gridtypes.Result{}

		assert.Equal(t, gateway.ZosWorkload(), gatewayWorkloadCp)
	})

	t.Run("test_workload_from_gateway_name", func(t *testing.T) {
		gatewayWorkloadCp := gatewayWorkload
		gatewayWorkloadCp.Result = gridtypes.Result{}

		workloadFromName, err := gateway.GenerateWorkloads()
		assert.NoError(t, err)
		assert.Equal(t, workloadFromName[0], gatewayWorkloadCp)
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		gatewayWorkloadCp := gatewayWorkload
		gatewayWorkloadCp.Result = gridtypes.Result{}

		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], gatewayWorkloadCp)

		workloadsMap2, err := gateway.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
