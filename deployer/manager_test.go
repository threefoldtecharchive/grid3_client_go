// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/json"

	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
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

func LoadZdbFromGrid(manager DeploymentManager, nodeID uint32, name string) (ZDB, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return ZDB{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}
	dataI, err := wl.WorkloadData()
	if err != nil {
		return ZDB{}, errors.Wrap(err, "failed to get workload data")
	}
	data, ok := dataI.(*zos.ZDB)
	if !ok {
		return ZDB{}, errors.New("couldn't cast workload data")
	}
	var result zos.ZDBResult
	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return ZDB{}, errors.Wrapf(err, "failed to get zdb result")
	}
	return ZDB{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Password:    data.Password,
		Public:      data.Public,
		Size:        int(data.Size / gridtypes.Gigabyte),
		Mode:        data.Mode.String(),
		IPs:         result.IPs,
		Port:        uint32(result.Port),
		Namespace:   result.Namespace,
	}, nil
}

func (z *ZDB) GenerateWorkloadFromZDB(zdb ZDB) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Name:        gridtypes.Name(zdb.Name),
		Type:        zos.ZDBType,
		Description: zdb.Description,
		Version:     0,
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     gridtypes.Unit(zdb.Size) * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode(zdb.Mode),
			Password: zdb.Password,
			Public:   zdb.Public,
		}),
	}, nil
}

func (z *ZDB) Stage(manager DeploymentManager, nodeID uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := z.GenerateWorkloadFromZDB(*z)
	if err != nil {
		panic(err)
	}
	workloads = append(workloads, workload)
	workloadsMap[nodeID] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}

type ZDB struct {
	Name        string
	Password    string
	Public      bool
	Size        int
	Description string
	Mode        string
	IPs         []string
	Port        uint32
	Namespace   string
}
