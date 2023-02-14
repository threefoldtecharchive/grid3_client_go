package deployer

import (
	"context"
	"encoding/json"
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
		tfPluginClient.twinID = twinID

		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl
		tfPluginClient.GridProxyClient = gridProxyCl

		tfPluginClient.StateLoader.ncPool = ncPool
		tfPluginClient.StateLoader.substrate = sub

		tfPluginClient.GatewayFQDNDeployer.deployer = deployer
		tfPluginClient.GatewayFQDNDeployer.tfPluginClient = &tfPluginClient
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

func mockValidation(identity substrate.Identity, cl *mocks.RMBMockClient, sub *mocks.MockSubstrateExt, ncPool *mocks.MockNodeClientGetter, proxyCl *mocks.MockClient) {
	sub.EXPECT().
		GetBalance(identity).
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
}

func TestValidateFQDNNodeReachable(t *testing.T) {
	d, cl, sub, ncPool, _, proxyCl := constructTestFQDNDeployer(t, true)

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)
	err := d.Validate(context.Background(), &workloads.GatewayFQDNProxy{Name: "test", NodeID: nodeID})
	assert.NoError(t, err)
}

func TestGenerateFQDNDeployment(t *testing.T) {
	d, _, _, _, _, _ := constructTestFQDNDeployer(t, true)
	gw := constructTestFQDN()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	testDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{
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
	})
	testDl.Metadata = "{\"type\":\"Gateway Fqdn\",\"name\":\"name\",\"projectName\":\"Gateway\"}"

	assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
		nodeID: testDl,
	})
}

func TestDeployFQDN(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)
	gw := constructTestFQDN()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{},
		dls,
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.NotEqual(t, gw.ContractID, 0)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestUpdateFQDN(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()

	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}
	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{nodeID: contractID},
		dls,
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestUpdateFQDNFailed(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{nodeID: contractID},
		dls,
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, errors.New("error"))

	err = d.Deploy(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestCancelFQDN(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Cancel(
		gomock.Any(),
		contractID,
	).Return(nil)

	err := d.Cancel(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, d.tfPluginClient.StateLoader.currentNodeDeployment, map[uint32]uint64{})
}

func TestCancelFQDNFailed(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	mockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Cancel(
		gomock.Any(),
		contractID,
	).Return(errors.New("error"))

	err := d.Cancel(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, d.tfPluginClient.StateLoader.currentNodeDeployment, map[uint32]uint64{nodeID: contractID})
}

func TestSyncFQDNContracts(t *testing.T) {
	d, _, sub, _, _, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	err := d.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, uint64(contractID))
}

func TestSyncFQDNDeletedContracts(t *testing.T) {
	d, _, sub, _, _, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).DoAndReturn(func(contracts map[uint32]uint64) error {
		delete(contracts, nodeID)
		return nil
	})

	err := d.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.ContractID, uint64(0))
}

func TestSyncFQDNContractsFailure(t *testing.T) {
	d, _, sub, _, _, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("error"))

	err := d.syncContracts(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, uint64(contractID))
}

func TestSyncFQDNFailureInContract(t *testing.T) {
	d, _, sub, _, _, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("error"))

	err := d.Sync(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, uint64(contractID))
}

func TestSyncFQDN(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	dl := dls[nodeID]
	dl.Workloads[0].Result.State = gridtypes.StateOk
	dl.Workloads[0].Result.Data, err = json.Marshal(zos.GatewayFQDNResult{})
	assert.NoError(t, err)

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{nodeID: contractID}).
		DoAndReturn(func(ctx context.Context, _ map[uint32]uint64) (map[uint32]gridtypes.Deployment, error) {
			return map[uint32]gridtypes.Deployment{nodeID: dl}, nil
		})

	gw.FQDN = "123"
	err = d.Sync(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, uint64(contractID))
	assert.Equal(t, gw.FQDN, "name.com")
}

func TestSyncFQDNDeletedWorkload(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestFQDNDeployer(t, true)

	gw := constructTestFQDN()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	dl := dls[nodeID]
	// state is deleted

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{nodeID: contractID}).
		DoAndReturn(func(ctx context.Context, _ map[uint32]uint64) (map[uint32]gridtypes.Deployment, error) {
			return map[uint32]gridtypes.Deployment{nodeID: dl}, nil
		})

	gw.FQDN = "123"
	err = d.Sync(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, uint64(contractID))
	assert.Equal(t, gw.FQDN, "")
	assert.Equal(t, gw.Name, "")
	assert.Equal(t, gw.TLSPassthrough, false)
	assert.Equal(t, gw.Backends, []zos.Backend([]zos.Backend(nil)))
}
