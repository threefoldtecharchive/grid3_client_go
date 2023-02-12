// Package provider is the terraform provider
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
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

var nameContractID uint64 = 200

func constructTestNameDeployer(t *testing.T, mock bool) (
	GatewayNameDeployer,
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

		tfPluginClient.GatewayNameDeployer.deployer = deployer
		tfPluginClient.GatewayNameDeployer.tfPluginClient = &tfPluginClient
	}

	return tfPluginClient.GatewayNameDeployer, cl, sub, ncPool, deployer, gridProxyCl
}

func constructTestName() workloads.GatewayNameProxy {
	return workloads.GatewayNameProxy{
		NodeID:         nodeID,
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"http://1.1.1.1", "http://2.2.2.2"},
		FQDN:           "name.com",
	}
}

func TestNameValidateNodeNotReachable(t *testing.T) {
	d, cl, sub, ncPool, _, _ := constructTestNameDeployer(t, true)
	sub.EXPECT().
		GetBalance(d.tfPluginClient.identity).
		Return(substrate.Balance{
			Free: types.U128{
				Int: big.NewInt(100000),
			},
		}, nil)
	cl.
		EXPECT().
		Call(
			gomock.Any(),
			nodeID,
			"zos.system.version",
			nil,
			gomock.Any(),
		).
		Return(errors.New("couldn't reach node"))
	ncPool.
		EXPECT().
		GetNodeClient(
			gomock.Any(),
			nodeID,
		).
		Return(client.NewNodeClient(nodeID, cl), nil)

	gatewayName := workloads.GatewayNameProxy{NodeID: nodeID}
	err := d.Validate(context.TODO(), &gatewayName)
	assert.Error(t, err)
}

func TestNameGenerateDeployment(t *testing.T) {
	d, _, _, _, _, _ := constructTestNameDeployer(t, true)
	g := constructTestName()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &g)
	assert.NoError(t, err)

	testDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{
		{
			Version: 0,
			Type:    zos.GatewayNameProxyType,
			Name:    gridtypes.Name(g.Name),
			Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
				TLSPassthrough: g.TLSPassthrough,
				Backends:       g.Backends,
				Name:           g.Name,
			}),
		},
	})
	testDl.Metadata = "{\"type\":\"Gateway Name\",\"name\":\"name.com\",\"projectName\":\"Gateway\"}"

	assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
		nodeID: testDl,
	})
}

func TestNameDeploy(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestNameDeployer(t, true)
	gw := constructTestName()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.identity, cl, sub, ncPool, proxyCl)

	newDeploymentsSolutionProvider := map[uint32]*uint64{nodeID: nil}

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{},
		dls,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{nodeID: contractID}, nil)

	sub.EXPECT().
		CreateNameContract(d.tfPluginClient.identity, gw.Name).
		Return(contractID, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestNameUpdate(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.NameContractID = nameContractID

	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{nodeID: contractID},
		dls,
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, nil)

	sub.EXPECT().
		InvalidateNameContract(gomock.Any(), d.tfPluginClient.identity, nameContractID, gw.Name).
		Return(nameContractID, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestNameUpdateFailed(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestNameDeployer(t, true)
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}
	gw := constructTestName()
	gw.NameContractID = nameContractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	mockValidation(d.tfPluginClient.identity, cl, sub, ncPool, proxyCl)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		map[uint32]uint64{nodeID: contractID},
		dls,
		gomock.Any(),
	).Return(map[uint32]uint64{nodeID: contractID}, errors.New("error"))

	sub.EXPECT().
		InvalidateNameContract(gomock.Any(), d.tfPluginClient.identity, nameContractID, gw.Name).
		Return(nameContractID, nil)

	err = d.Deploy(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.NameContractID, nameContractID)
}

func TestNameCancel(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestNameDeployer(t, true)
	gw := constructTestName()
	gw.NameContractID = nameContractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	deployer.EXPECT().Cancel(
		gomock.Any(),
		contractID,
	).Return(nil)

	sub.EXPECT().
		EnsureContractCanceled(d.tfPluginClient.identity, nameContractID).
		Return(nil)

	err := d.Cancel(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.NameContractID, uint64(0))
}

func TestNameCancelDeploymentsFailed(t *testing.T) {
	d, _, _, _, deployer, _ := constructTestNameDeployer(t, true)
	gw := constructTestName()
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	deployer.EXPECT().Cancel(
		gomock.Any(),
		contractID,
	).Return(errors.New("error"))

	err := d.Cancel(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}

func TestNameCancelContractsFailed(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.NameContractID = nameContractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}

	deployer.EXPECT().Cancel(
		gomock.Any(),
		contractID,
	).Return(nil)

	sub.EXPECT().
		EnsureContractCanceled(d.tfPluginClient.identity, nameContractID).
		Return(errors.New("error"))

	err := d.Cancel(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Empty(t, d.tfPluginClient.StateLoader.currentNodeDeployment)
	assert.Equal(t, gw.NameContractID, nameContractID)
}

func TestNameSyncContracts(t *testing.T) {
	d, _, sub, _, _, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(true, nil)

	err := d.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.ContractID, contractID)
}

func TestNameSyncDeletedContracts(t *testing.T) {
	d, _, sub, _, _, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).DoAndReturn(func(contracts map[uint32]uint64) error {
		delete(contracts, nodeID)
		return nil
	})

	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(false, nil)

	err := d.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.NameContractID, uint64(0))
	assert.Equal(t, gw.ContractID, uint64(0))
}

func TestNameSyncContractsFailure(t *testing.T) {
	d, _, sub, _, _, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.ContractID = contractID
	gw.NameContractID = nameContractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("error"))

	err := d.syncContracts(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.NameContractID, nameContractID)
	assert.Equal(t, gw.ContractID, contractID)
}

func TestNameSync(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.ContractID = contractID
	gw.NameContractID = nameContractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	dl := dls[nodeID]

	dl.Workloads[0].Result.State = gridtypes.StateOk
	dl.Workloads[0].Result.Data, err = json.Marshal(zos.GatewayProxyResult{FQDN: "name.com"})
	assert.NoError(t, err)

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(true, nil)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{nodeID: contractID}).
		Return(map[uint32]gridtypes.Deployment{nodeID: dl}, nil)

	gw.FQDN = "123"
	err = d.Sync(context.Background(), &gw)
	assert.Equal(t, gw.FQDN, "name.com")
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	assert.Equal(t, gw.NameContractID, nameContractID)
	assert.Equal(t, gw.ContractID, contractID)
}

func TestNameSyncDeletedWorkload(t *testing.T) {
	d, _, sub, _, deployer, _ := constructTestNameDeployer(t, true)

	gw := constructTestName()
	gw.ContractID = contractID
	gw.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)
	dl := dls[nodeID]
	// state is deleted

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)

	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(true, nil)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{nodeID: contractID}).
		Return(map[uint32]gridtypes.Deployment{nodeID: dl}, nil)

	gw.FQDN = "123"
	err = d.Sync(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Empty(t, gw.Backends)
	assert.Empty(t, gw.TLSPassthrough)
	assert.Empty(t, gw.Name)
	assert.Empty(t, gw.FQDN)
	assert.Equal(t, gw.ContractID, contractID)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
}
