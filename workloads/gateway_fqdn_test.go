package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	mock_deployer "github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayFQDNStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := mock_deployer.NewMockDeploymentManager(ctrl)

	gateway := GatewayFQDNProxy{
		Name:           "test",
		TLSPassthrough: true,
		Backends:       []zos.Backend{"http://1.1.1.1"},
		FQDN:           "test",
	}
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
	wlMap := map[uint32][]gridtypes.Workload{}
	wlMap[1] = append(wlMap[1], gatewayWl)
	manager.EXPECT().SetWorkloads(gomock.Eq(wlMap)).Return(nil)
	err := gateway.Stage(manager, 1)
	assert.NoError(t, err)
}
