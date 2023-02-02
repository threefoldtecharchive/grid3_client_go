package deployer

import (
	"context"
	"math/big"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	proxyTypes "github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func constructTestFQDNDeployer(t *testing.T, mock bool) (
	GatewayFQDNDeployer,
	*mocks.RMBMockClient,
	*mocks.MockSubstrateExt,
	*mocks.MockNodeClientGetter,
	*mocks.MockDeployer,
	*mocks.MockClient,
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	deployer := mocks.NewMockDeployer(ctrl)
	gridProxyCl := mocks.NewMockClient(ctrl)

	if mock {
		tfPluginClient.TwinID = twinID

		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl
		tfPluginClient.GridProxyClient = gridProxyCl

		tfPluginClient.GatewayFQDNDeployer.deployer = deployer

		tfPluginClient.StateLoader.ncPool = ncPool
		tfPluginClient.StateLoader.substrate = sub
	}

	return tfPluginClient.GatewayFQDNDeployer, cl, sub, ncPool, deployer, gridProxyCl
}

func constructTestFQDN() workloads.GatewayFQDNProxy {
	return workloads.GatewayFQDNProxy{
		NodeID:         nodeID,
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"http://1.1.1.1", "http://2.2.2.2"},
		FQDN:           "name.com",
	}
}

func TestValidateNodeReachable(t *testing.T) {
	d, cl, sub, ncPool, _, _ := constructTestFQDNDeployer(t, true)

	sub.EXPECT().
		GetBalance(d.tfPluginClient.Identity).
		Return(substrate.Balance{
			Free: types.U128{
				Int: big.NewInt(100000),
			},
		}, nil)

	cl.EXPECT().
		Call(
			gomock.Any(),
			twinID,
			"zos.system.version",
			nil,
			gomock.Any(),
		).
		Return(nil)

	ncPool.EXPECT().
		GetNodeClient(
			gomock.Any(),
			nodeID,
		).
		Return(client.NewNodeClient(twinID, cl), nil)

	err := d.Validate(context.TODO(), &workloads.GatewayFQDNProxy{NodeID: nodeID})
	assert.NoError(t, err)
}

func TestGenerateDeployment(t *testing.T) {
	d, _, _, _, _, _ := constructTestFQDNDeployer(t, false)
	gw := constructTestFQDN()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
		10: {
			Version: 0,
			TwinID:  d.tfPluginClient.TwinID,
			Workloads: []gridtypes.Workload{
				{
					Version: 0,
					Type:    zos.GatewayFQDNProxyType,
					Name:    gridtypes.Name(gw.Name),
					Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
						TLSPassthrough: gw.TLSPassthrough,
						Backends:       gw.Backends,
						FQDN:           gw.FQDN,
					}),
				},
			},
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: d.tfPluginClient.TwinID,
						Weight: 1,
					},
				},
			},
		},
	})
}

func TestDeployFQDN(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)
	d.tfPluginClient.TwinID = twinID

	gw := constructTestFQDN()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	sub.EXPECT().
		GetBalance(d.tfPluginClient.Identity).
		Return(substrate.Balance{
			Free: types.U128{
				Int: big.NewInt(100000),
			},
		}, nil)

	proxyCl.EXPECT().Node(nodeID).
		Return(proxyTypes.NodeWithNestedCapacity{
			NodeID:       int(nodeID),
			FarmID:       1,
			PublicConfig: proxyTypes.PublicConfig{Ipv4: "1.1.1.1/16"},
		}, nil)

	proxyCl.EXPECT().Farms(gomock.Any(), gomock.Any()).Return([]proxyTypes.Farm{{FarmID: 1}}, 1, nil)

	ncPool.EXPECT().
		GetNodeClient(sub, nodeID).AnyTimes().
		Return(client.NewNodeClient(twinID, cl), nil)

	cl.EXPECT().Call(
		gomock.Any(),
		twinID,
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		nil,
		dls,
		gomock.Any(),
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.NotEqual(t, gw.ContractID, 0)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

/*
func TestUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	ncPool := mock.NewMockNodeClientGetter(ctrl)
	gw := GatewayFQDNDeployer{
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		ncPool:           ncPool,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		dls,
	).Return(map[uint32]uint64{uint32(10): uint64(100)}, nil)
	ncPool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().Call(
		gomock.Any(),
		uint32(12),
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)
	err = gw.Deploy(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
}
*/

func TestUpdateFailed(t *testing.T) {
	d, cl, sub, ncPool, deployer, _ := constructTestFQDNDeployer(t, true)
	d.tfPluginClient.TwinID = twinID

	gw := workloads.GatewayFQDNProxy{
		NodeID:         nodeID,
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:           "name.com",
	}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{nodeID: contractID},
		dls,
		gomock.Any(),
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, errors.New("error"))

	ncPool.EXPECT().
		GetNodeClient(sub, nodeID).
		Return(client.NewNodeClient(12, cl), nil)

	cl.EXPECT().Call(
		gomock.Any(),
		twinID,
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	err = d.Deploy(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

/*
func TestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		map[uint32]gridtypes.Deployment{},
	).Return(map[uint32]uint64{}, nil)
	err = gw.Cancel(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
}

func TestCancelFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		map[uint32]gridtypes.Deployment{},
	).Return(map[uint32]uint64{10: 100}, errors.New("error"))
	err = gw.Cancel(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
}

func TestSyncContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)
	err = gw.syncContracts(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSyncDeletedContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).DoAndReturn(func(contracts map[uint32]uint64) error {
		delete(contracts, 10)
		return nil
	})
	err = gw.syncContracts(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.ID, "")
}

func TestSyncContractsFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("123"))
	err = gw.syncContracts(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSyncFailureInContract(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	deployer := mock.NewMockDeployer(ctrl)
	assert.NoError(t, err)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
	}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("123"))
	err = gw.Sync(context.Background(), sub, gw.ThreefoldPluginClient)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	ncPool := mock.NewMockNodeClientGetter(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
		ncPool:           ncPool,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	dl := dls[10]
	dl.Workloads[0].Result.State = gridtypes.StateOk
	dl.Workloads[0].Result.Data, err = json.Marshal(zos.GatewayFQDNResult{})
	assert.NoError(t, err)

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)
	deployer.EXPECT().
		GetDeployments(gomock.Any(), sub, map[uint32]uint64{10: 100}).
		DoAndReturn(func(ctx context.Context, _ subi.SubstrateExt, _ map[uint32]uint64) (map[uint32]gridtypes.Deployment, error) {
			return map[uint32]gridtypes.Deployment{10: dl}, nil
		})
	gw.Gw.FQDN = "123"
	err = gw.Sync(context.Background(), sub, gw.ThreefoldPluginClient)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
	assert.Equal(t, gw.Gw.FQDN, "name.com")
}

func TestSyncDeletedWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	ncPool := mock.NewMockNodeClientGetter(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		ThreefoldPluginClient: &threefoldPluginClient{
			identity: identity,
			twinID:   11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
		ncPool:           ncPool,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	dl := dls[10]
	// state is deleted

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)
	deployer.EXPECT().
		GetDeployments(gomock.Any(), sub, map[uint32]uint64{10: 100}).
		DoAndReturn(func(ctx context.Context, _ subi.SubstrateExt, _ map[uint32]uint64) (map[uint32]gridtypes.Deployment, error) {
			return map[uint32]gridtypes.Deployment{10: dl}, nil
		})
	gw.Gw.FQDN = "123"
	err = gw.Sync(context.Background(), sub, gw.ThreefoldPluginClient)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
	assert.Equal(t, gw.Gw, workloads.GatewayFQDNProxy{})
}
*/
