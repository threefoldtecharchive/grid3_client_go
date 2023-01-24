// Package deployer for grid deployer
package deployer

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	proxyTypes "github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestValidator(t *testing.T) {
	_, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gridClient := mocks.NewMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)

	validatorImpl := ValidatorImpl{
		gridClient: gridClient,
	}

	dl1 := workloads.NewDeployment(twinID, []gridtypes.Workload{})

	newDls := map[uint32]gridtypes.Deployment{
		10: dl1,
	}

	gridClient.EXPECT().
		Node(uint32(10)).
		Return(proxyTypes.NodeWithNestedCapacity{
			FarmID: 1,
			PublicConfig: proxyTypes.PublicConfig{
				Domain: "test",
			},
		}, nil)

	farmID := uint64(1)
	gridClient.EXPECT().
		Farms(proxyTypes.FarmFilter{
			FarmID: &farmID,
		}, proxyTypes.Limit{
			Page: 1,
			Size: 1,
		}).
		Return([]proxyTypes.Farm{{FarmID: 1}}, 1, nil)

	err = validatorImpl.Validate(
		context.Background(),
		sub,
		nil,
		newDls,
	)

	assert.NoError(t, err)
}

func TestCapacityHelpers(t *testing.T) {
	cap := gridtypes.Capacity{
		CRU: 1,
		SRU: 2,
		HRU: 3,
		MRU: 4,
	}

	t.Run("capacity print", func(t *testing.T) {
		capPrint := "[mru: 4, sru: 2, hru: 3]"
		assert.Equal(t, capPrint, capacityPrettyPrint(cap))
	})

	t.Run("capacity add", func(t *testing.T) {
		originalCap := proxyTypes.Capacity{
			CRU: 1,
			SRU: 2,
			HRU: 3,
			MRU: 4,
		}

		addCapacity(&originalCap, &cap)
		assert.Equal(t, originalCap.CRU, uint64(2))
		assert.Equal(t, originalCap.SRU, gridtypes.Unit(4))
		assert.Equal(t, originalCap.HRU, gridtypes.Unit(6))
		assert.Equal(t, originalCap.MRU, gridtypes.Unit(8))
	})
}
