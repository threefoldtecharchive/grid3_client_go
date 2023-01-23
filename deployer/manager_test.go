// Package deployer for grid deployer
package deployer

import (
	"context"

	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestCancelAll(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		nodeID,
		dMap,
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().
		CancelContract(
			identity,
			uint64(100),
		).Return(nil)
	err := manager.CancelAll()
	assert.NoError(t, err)
}

func TestCommit(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	dl1.ContractID = 100
	manager := NewDeploymentManager(
		identity,
		nodeID,
		map[uint32]uint64{10: 100},
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().Close()
	err := manager.Commit(context.Background())
	assert.NoError(t, err)
}

func TestSetWorkload(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	zdbWl := gridtypes.Workload{
		Name:        gridtypes.Name("test"),
		Type:        zos.ZDBType,
		Description: "test des",
		Version:     0,
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     100 * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode("user"),
			Password: "password",
			Public:   true,
		}),
	}
	wlMap := map[uint32][]gridtypes.Workload{}
	wlMap[1] = append(wlMap[1], zdbWl)
	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		nodeID,
		dMap,
		gridClient,
		ncPool,
		subi,
	)
	ncPool.EXPECT().
		GetNodeClient(
			sub,
			uint32(10),
		).Return(client.NewNodeClient(13, cl), nil).AnyTimes()
	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		}).AnyTimes()

	err := manager.SetWorkloads(wlMap)
	assert.NoError(t, err)
}
func TestCancelWorkloads(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	gridClient := mocks.NewMockClient(ctrl)

	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	dl1.ContractID = 100
	wlMap := map[uint32]map[string]bool{
		10: {
			"name": true,
		},
	}
	manager := NewDeploymentManager(
		identity,
		nodeID,
		map[uint32]uint64{10: 100},
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().Close()
	ncPool.EXPECT().
		GetNodeClient(
			sub,
			uint32(10),
		).Return(client.NewNodeClient(13, cl), nil)
	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		})

	err := manager.CancelWorkloads(wlMap)
	assert.NoError(t, err)
}
func TestGetWorkload(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	gw := GatewayNameProxy{
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backendURLWithoutTLSPassthrough)},
	}

	_, err := gw.GenerateWorkloadFromGName(gw)
	assert.NoError(t, err)
	dl1.ContractID = 100
	manager := NewDeploymentManager(
		identity,
		nodeID,
		map[uint32]uint64{10: 100},
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().Close()
	ncPool.EXPECT().
		GetNodeClient(
			sub,
			uint32(10),
		).Return(client.NewNodeClient(13, cl), nil)
	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		})
	_, err = manager.GetWorkload(uint32(10), "name")
	assert.NoError(t, err)

}

func TestGetDeployment(t *testing.T) {
	identity, nodeID := setUP()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	gridClient := mocks.NewMockClient(ctrl)

	dl1 := deployment1(identity, false, 0, backendURLWithoutTLSPassthrough)
	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		nodeID,
		dMap,
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().Close()
	ncPool.EXPECT().
		GetNodeClient(
			sub,
			uint32(10),
		).Return(client.NewNodeClient(13, cl), nil)
	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		})
	_, err := manager.GetDeployment(uint32(10))
	assert.NoError(t, err)
}
