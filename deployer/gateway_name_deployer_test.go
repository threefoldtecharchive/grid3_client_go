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
	mock "github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestNameValidateNodeNotReachable(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	pool := mock.NewMockNodeClientGetter(ctrl)
	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	sub.
		EXPECT().
		GetBalance(
			identity,
		).
		Return(substrate.Balance{Free: types.NewU128(*big.NewInt(20000))}, nil)
	cl.
		EXPECT().
		Call(
			gomock.Any(),
			uint32(10),
			"zos.system.version",
			nil,
			gomock.Any(),
		).
		Return(errors.New("couldn't reach node"))
	pool.
		EXPECT().
		GetNodeClient(
			gomock.Any(),
			uint32(10),
		).
		Return(client.NewNodeClient(10, cl), nil)

	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		NcPool:        pool,
		SubstrateConn: sub,
	})
	gatewayName := workloads.GatewayNameProxy{NodeID: 10}
	err = gatewayNameDeployer.Validate(context.TODO(), &gatewayName)
	assert.Error(t, err)
}

func TestNameValidateNodeReachable(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	pool := mock.NewMockNodeClientGetter(ctrl)
	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	sub.
		EXPECT().
		GetBalance(
			identity,
		).
		Return(substrate.Balance{Free: types.NewU128(*big.NewInt(20000))}, nil)
	cl.
		EXPECT().
		Call(
			gomock.Any(),
			uint32(10),
			"zos.system.version",
			nil,
			gomock.Any(),
		).
		Return(nil)
	pool.
		EXPECT().
		GetNodeClient(
			gomock.Any(),
			uint32(10),
		).
		Return(client.NewNodeClient(10, cl), nil)

	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		NcPool:        pool,
		SubstrateConn: sub,
	})

	gatewayName := workloads.GatewayNameProxy{NodeID: 10}
	err = gatewayNameDeployer.Validate(context.TODO(), &gatewayName)
	assert.NoError(t, err)
}

func TestNameGenerateDeployment(t *testing.T) {
	g := workloads.GatewayNameProxy{
		NodeID: 10,

		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"a", "b"},
		FQDN:           "name.com",
	}

	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		TwinID: 11,
	})

	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &g)
	assert.NoError(t, err)
	assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
		10: {
			Version: 0,
			TwinID:  11,
			Workloads: []gridtypes.Workload{
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
			},
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: 11,
						Weight: 1,
					},
				},
			},
		},
	})
}

func TestNameDeploy(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	pool := mock.NewMockNodeClientGetter(ctrl)

	gw := workloads.GatewayNameProxy{
		NodeID:         10,
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:           "name.com",
	}
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		NcPool:        pool,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer

	newDeploymentsSolutionProvider := map[uint32]*uint64{10: nil}
	deploymentData := workloads.DeploymentData{
		Name: gw.Name,
		Type: "Gateway Name",
	}
	newDeploymentsData := map[uint32]workloads.DeploymentData{10: deploymentData}
	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	sub.
		EXPECT().
		GetBalance(
			identity,
		).
		Return(substrate.Balance{Free: types.NewU128(*big.NewInt(20000))}, nil)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		nil,
		dls,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{10: 100}, nil)
	sub.EXPECT().
		CreateNameContract(identity, "name").
		Return(uint64(100), nil)
	pool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().Call(
		gomock.Any(),
		uint32(12),
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)
	err = gatewayNameDeployer.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
}

func TestNameUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	pool := mock.NewMockNodeClientGetter(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		NcPool:        pool,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	newDeploymentsSolutionProvider := map[uint32]*uint64{10: nil}
	deploymentData := workloads.DeploymentData{
		Name: gw.Name,
		Type: "Gateway Name",
	}
	newDeploymentsData := map[uint32]workloads.DeploymentData{10: deploymentData}
	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	sub.
		EXPECT().
		GetBalance(
			identity,
		).
		Return(substrate.Balance{Free: types.NewU128(*big.NewInt(20000))}, nil)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		gw.NodeDeploymentID,
		dls,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{10: 100}, nil)
	sub.EXPECT().
		InvalidateNameContract(gomock.Any(), identity, uint64(200), gw.Name).
		Return(uint64(200), nil)

	pool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().Call(
		gomock.Any(),
		uint32(12),
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)
	err = gatewayNameDeployer.Deploy(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
}

func TestNameUpdateFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	pool := mock.NewMockNodeClientGetter(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		NcPool:        pool,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	newDeploymentsSolutionProvider := map[uint32]*uint64{10: nil}
	deploymentData := workloads.DeploymentData{
		Name: gw.Name,
		Type: "Gateway Name",
	}
	newDeploymentsData := map[uint32]workloads.DeploymentData{10: deploymentData}
	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)

	sub.
		EXPECT().
		GetBalance(
			identity,
		).
		Return(substrate.Balance{Free: types.NewU128(*big.NewInt(20000))}, nil)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		gw.NodeDeploymentID,
		dls,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{10: 100}, errors.New("error"))
	sub.EXPECT().
		InvalidateNameContract(gomock.Any(), identity, uint64(200), gw.Name).
		Return(uint64(200), nil)
	pool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().Call(
		gomock.Any(),
		uint32(12),
		"zos.system.version",
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	err = gatewayNameDeployer.Deploy(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
	assert.Equal(t, gw.NameContractID, uint64(200))
}

func TestNameCancel(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	newDeployments := make(map[uint32]gridtypes.Deployment)
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	assert.NoError(t, err)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		gw.NodeDeploymentID,
		newDeployments,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{}, nil)
	sub.EXPECT().
		EnsureContractCanceled(identity, uint64(200)).
		Return(nil)

	err = gatewayNameDeployer.Cancel(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.NameContractID, uint64(0))
}

func TestNameCancelDeploymentsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	newDeployments := make(map[uint32]gridtypes.Deployment)
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	assert.NoError(t, err)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		gw.NodeDeploymentID,
		newDeployments,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{10: 100}, errors.New("error"))
	err = gatewayNameDeployer.Cancel(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
}

func TestNameCancelContractsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	newDeployments := make(map[uint32]gridtypes.Deployment)
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	assert.NoError(t, err)

	deployer.EXPECT().Deploy(
		gomock.Any(),
		gw.NodeDeploymentID,
		newDeployments,
		newDeploymentsData,
		newDeploymentsSolutionProvider,
	).Return(map[uint32]uint64{}, nil)
	sub.EXPECT().
		EnsureContractCanceled(identity, uint64(200)).
		Return(errors.New("error"))

	err = gatewayNameDeployer.Cancel(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.NameContractID, uint64(200))
}

func TestNameSyncContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		ContractID:       100,
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)
	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(true, nil)

	err = gatewayNameDeployer.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ContractID, uint64(100))
}

func TestNameSyncDeletedContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		ContractID:       100,
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).DoAndReturn(func(contracts map[uint32]uint64) error {
		delete(contracts, 10)
		return nil
	})
	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(false, nil)
	err = gatewayNameDeployer.syncContracts(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.NameContractID, uint64(0))
	assert.Equal(t, gw.ContractID, uint64(0))
}

func TestNameSyncContractsFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		ContractID:       100,
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(errors.New("123"))

	err = gatewayNameDeployer.syncContracts(context.Background(), &gw)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.NameContractID, uint64(200))
	assert.Equal(t, gw.ContractID, uint64(100))
}

func TestNameSync(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		ContractID:       100,
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)
	dl := dls[10]
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
		GetDeployments(gomock.Any(), map[uint32]uint64{10: 100}).
		Return(map[uint32]gridtypes.Deployment{10: dl}, nil)
	gw.FQDN = "123"
	err = gatewayNameDeployer.sync(context.Background(), &gw)
	assert.Equal(t, gw.FQDN, "name.com")
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.NameContractID, uint64(200))
	assert.Equal(t, gw.ContractID, uint64(100))
}

func TestNameSyncDeletedWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase("")
	assert.NoError(t, err)
	deployer := mock.NewMockDeployer(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	gatewayNameDeployer := NewGatewayNameDeployer(&TFPluginClient{
		Identity:      identity,
		SubstrateConn: sub,
		TwinID:        11,
	})
	gatewayNameDeployer.deployer = deployer
	gw := workloads.GatewayNameProxy{
		ContractID:       100,
		NodeID:           10,
		NodeDeploymentID: map[uint32]uint64{10: 100},
		NameContractID:   200,
		Name:             "name",
		TLSPassthrough:   false,
		Backends:         []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
		FQDN:             "name.com",
	}
	dls, err := gatewayNameDeployer.GenerateVersionlessDeployments(context.Background(), &gw)
	assert.NoError(t, err)
	dl := dls[10]
	// state is deleted

	sub.EXPECT().DeleteInvalidContracts(
		gw.NodeDeploymentID,
	).Return(nil)
	sub.EXPECT().IsValidContract(
		gw.NameContractID,
	).Return(true, nil)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{10: 100}).
		Return(map[uint32]gridtypes.Deployment{10: dl}, nil)
	gw.FQDN = "123"
	err = gatewayNameDeployer.sync(context.Background(), &gw)
	assert.NoError(t, err)
	assert.Equal(t, gw, workloads.GatewayNameProxy{})
}
