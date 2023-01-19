// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayNameProxyWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gatewayName := "test"
	gatewayZosBackend := zos.Backend("http://1.1.1.1")

	var gateway GatewayNameProxy

	gatewayWorkload := gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(gatewayName),
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           gatewayName,
			TLSPassthrough: true,
			Backends:       []zos.Backend{gatewayZosBackend},
		}),
	}

	t.Run("test_gateway_from_zos_workload", func(t *testing.T) {
		var err error

		gateway, err = GatewayNameProxyFromZosWorkload(gatewayWorkload)
		assert.NoError(t, err)
	})

	t.Run("test_gateway_functions", func(t *testing.T) {
		assert.Equal(t, gateway.ZosWorkload(), gatewayWorkload)
	})

	t.Run("test_workload_from_gateway_name", func(t *testing.T) {
		workloadFromFQDN, err := gateway.GenerateWorkloadFromGName()
		assert.NoError(t, err)
		assert.Equal(t, workloadFromFQDN, gatewayWorkload)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], gatewayWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := gateway.Stage(manager, nodeID)
		assert.NoError(t, err)
	})
}