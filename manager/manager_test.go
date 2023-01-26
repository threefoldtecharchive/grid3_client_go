// Package manager for grid manager
package manager

import (
	"context"
	"os"

	"testing"

	"github.com/golang/mock/gomock"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

var (
	SubstrateURLs = map[string]string{
		"dev":  "wss://tfchain.dev.grid.tf/ws",
		"test": "wss://tfchain.test.grid.tf/ws",
		"qa":   "wss://tfchain.qa.grid.tf/ws",
		"main": "wss://tfchain.grid.tf/ws",
	}
)

var backendURLWithoutTLSPassthrough = "http://1.1.1.1:10"

func SetUP() (identity subi.Identity, twinID uint32, err error) {
	if _, err = os.Stat("../.env"); !errors.Is(err, os.ErrNotExist) {
		err = godotenv.Load("../.env")
		if err != nil {
			return
		}
	}

	mnemonics := os.Getenv("MNEMONICS")
	identity, err = substrate.NewIdentityFromSr25519Phrase(mnemonics)
	if err != nil {
		return
	}

	keyPair, err := identity.KeyPair()
	if err != nil {
		return
	}

	network := os.Getenv("NETWORK")
	pub := keyPair.Public()
	sub := subi.NewManager(SubstrateURLs[network])
	subext, err := sub.SubstrateExt()
	if err != nil {
		return
	}
	twin, err := subext.GetTwinByPubKey(pub)
	if err != nil {
		return
	}
	return identity, twin, nil
}

func deploymentWithNameGateway(identity substrate.Identity, twinID uint32, TLSPassthrough bool, version uint32, backendURL string) (gridtypes.Deployment, error) {
	gw := workloads.GatewayNameProxy{
		Name:           "name",
		TLSPassthrough: TLSPassthrough,
		Backends:       []zos.Backend{zos.Backend(backendURL)},
	}

	return workloads.NewDeploymentWithGateway(identity, twinID, version, &gw)
}

func TestCancelAll(t *testing.T) {
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		twinID,
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
	err = manager.CancelAll()
	assert.NoError(t, err)
}

func TestCommit(t *testing.T) {
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100
	manager := NewDeploymentManager(
		identity,
		twinID,
		map[uint32]uint64{10: 100},
		gridClient,
		ncPool,
		subi,
	)
	subi.EXPECT().
		SubstrateExt().
		Return(sub, nil)
	sub.EXPECT().Close()
	err = manager.Commit(context.Background())
	assert.NoError(t, err)
}

func TestSetWorkload(t *testing.T) {
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

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
	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		twinID,
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

	err = manager.SetWorkloads(wlMap)
	assert.NoError(t, err)
}
func TestCancelWorkloads(t *testing.T) {
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	gridClient := mocks.NewMockClient(ctrl)

	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100
	wlMap := map[uint32]map[string]bool{
		10: {
			"name": true,
		},
	}
	manager := NewDeploymentManager(
		identity,
		twinID,
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

	err = manager.CancelWorkloads(wlMap)
	assert.NoError(t, err)
}
func TestGetWorkload(t *testing.T) {
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	gridClient := mocks.NewMockClient(ctrl)
	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	gw := workloads.GatewayNameProxy{
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backendURLWithoutTLSPassthrough)},
	}

	_, err = gw.GenerateWorkloads()
	assert.NoError(t, err)
	dl1.ContractID = 100
	manager := NewDeploymentManager(
		identity,
		twinID,
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
	identity, twinID, err := SetUP()
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sub := mocks.NewMockSubstrateExt(ctrl)
	subi := mocks.NewMockManagerInterface(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	cl := mocks.NewRMBMockClient(ctrl)
	gridClient := mocks.NewMockClient(ctrl)

	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100
	dMap := map[uint32]uint64{
		10: 100,
	}
	manager := NewDeploymentManager(
		identity,
		twinID,
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
	_, err = manager.GetDeployment(uint32(10))
	assert.NoError(t, err)
}
