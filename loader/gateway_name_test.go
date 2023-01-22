// Package loader to load different types, workloads from grid
package loader

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadGatewayNameFromGrid(t *testing.T) {
	res, _ := json.Marshal(zos.GatewayProxyResult{
		FQDN: "test fqdn",
	})
	gatewayWl := gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name("test"),
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           "test",
			TLSPassthrough: true,
			Backends:       []zos.Backend{"http://1.1.1.1"},
		}),
		Result: gridtypes.Result{
			Created: 1000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
	}
	gateway := workloads.GatewayNameProxy{
		Name:           "test",
		TLSPassthrough: true,
		Backends:       []zos.Backend{"http://1.1.1.1"},
		FQDN:           "test fqdn",
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWl, nil)
		got, err := LoadGatewayNameFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, gateway, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		gatewayWlCp := gatewayWl
		gatewayWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWlCp, nil)
		_, err := LoadGatewayNameFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		gatewayWlCp := gatewayWl
		gatewayWlCp.Type = zos.GatewayFQDNProxyType
		gatewayWlCp.Data = gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			FQDN: "123",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWlCp, nil)

		_, err := LoadGatewayNameFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
