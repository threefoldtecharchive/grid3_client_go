package loader

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	mock_deployer "github.com/threefoldtech/grid3-go/tests/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadGatewayFqdnFromGrid(t *testing.T) {
	gatewayWl := gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayFQDNProxyType,
		Name:    gridtypes.Name("test"),
		Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			TLSPassthrough: true,
			Backends:       []zos.Backend{"http://1.1.1.1"},
			FQDN:           "test",
		}),
	}
	gateway := workloads.GatewayFQDNProxy{
		Name:           "test",
		TLSPassthrough: true,
		Backends:       []zos.Backend{"http://1.1.1.1"},
		FQDN:           "test",
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWl, nil)
		got, err := loader.LoadGatewayFqdnFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, gateway, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		gatewayWlCp := gatewayWl
		gatewayWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWlCp, nil)
		_, err := loader.LoadGatewayFqdnFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		gatewayWlCp := gatewayWl
		gatewayWlCp.Type = zos.GatewayNameProxyType
		gatewayWlCp.Data = gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name: "name",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(gatewayWlCp, nil)

		_, err := loader.LoadGatewayFqdnFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
