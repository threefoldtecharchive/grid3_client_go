// Package loader to load different types, workloads from grid
package loader

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadNetworkFromGrid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := mocks.NewMockDeploymentManager(ctrl)

	ipRange, err := gridtypes.ParseIPNet("1.1.1.1/24")
	assert.NoError(t, err)

	znet := workloads.ZNet{
		Name:        "test",
		Description: "test description",
		Nodes:       []uint32{1},
		IPRange:     ipRange,
		AddWGAccess: false,
		ContractID:  1,
	}

	networkWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test"),
		Type:    zos.NetworkType,
		Data: gridtypes.MustMarshal(zos.Network{
			NetworkIPRange: gridtypes.MustParseIPNet(znet.IPRange.String()),
			Subnet:         ipRange,
			WGPrivateKey:   "",
			WGListenPort:   0,
			Peers:          []zos.Peer{},
		}),
		Metadata:    "",
		Description: "test description",
		Result:      gridtypes.Result{},
	}

	t.Run("success", func(t *testing.T) {
		dl := gridtypes.Deployment{
			Workloads: []gridtypes.Workload{networkWl},
		}
		manager.EXPECT().GetContractIDs().Return(map[uint32]uint64{1: 1})
		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)

		got, err := LoadNetworkFromGrid(manager, "test")
		assert.NoError(t, err)
		assert.Equal(t, znet, got)
	})

	t.Run("invalid type", func(t *testing.T) {
		networkWlCp := networkWl
		networkWlCp.Type = "invalid"

		dl := gridtypes.Deployment{
			Workloads: []gridtypes.Workload{networkWlCp},
		}

		manager.EXPECT().GetContractIDs().Return(map[uint32]uint64{1: 1})
		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)

		_, err := LoadNetworkFromGrid(manager, "test")
		assert.Error(t, err)
	})

	t.Run("wrong workload data", func(t *testing.T) {
		networkWlCp := networkWl
		networkWlCp.Type = zos.GatewayNameProxyType
		networkWlCp.Data = gridtypes.MustMarshal(zos.Network{
			WGPrivateKey: "key",
		})

		dl := gridtypes.Deployment{
			Workloads: []gridtypes.Workload{networkWlCp},
		}

		manager.EXPECT().GetContractIDs().Return(map[uint32]uint64{1: 1})
		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)

		_, err := LoadNetworkFromGrid(manager, "test")
		assert.Error(t, err)
	})
}
