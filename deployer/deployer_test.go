// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/hex"
	"encoding/json"
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

func SetUP() (identity substrate.Identity, twinID uint32, err error) {
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

var backendURLWithTLSPassthrough = "//1.1.1.1:10"
var backendURLWithoutTLSPassthrough = "http://1.1.1.1:10"

func deploymentWithNameGateway(identity substrate.Identity, twinID uint32, TLSPassthrough bool, version uint32, backendURL string) (gridtypes.Deployment, error) {
	gw := workloads.GatewayNameProxy{
		Name:           "name",
		TLSPassthrough: TLSPassthrough,
		Backends:       []zos.Backend{zos.Backend(backendURL)},
	}

	return workloads.NewDeploymentWithGateway(identity, twinID, version, &gw)
}

func deploymentWithFQDN(identity substrate.Identity, twinID uint32, version uint32) (gridtypes.Deployment, error) {
	gw := workloads.GatewayFQDNProxy{
		Name:     "fqdn",
		FQDN:     "a.b.com",
		Backends: []zos.Backend{zos.Backend(backendURLWithoutTLSPassthrough)},
	}

	return workloads.NewDeploymentWithGateway(identity, twinID, version, &gw)
}

func hash(dl *gridtypes.Deployment) (string, error) {
	hash, err := dl.ChallengeHash()
	if err != nil {
		return "", err
	}
	hashHex := hex.EncodeToString(hash)
	return hashHex, nil
}

type EmptyValidator struct{}

func (d *EmptyValidator) Validate(ctx context.Context, sub subi.SubstrateExt, oldDeployments map[uint32]gridtypes.Deployment, newDeployments map[uint32]gridtypes.Deployment) error {
	return nil
}

func TestCreate(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	identity := tfPluginClient.Identity
	twinID := tfPluginClient.TwinID

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	tfPluginClient.SubstrateConn = sub
	tfPluginClient.NcPool = ncPool
	tfPluginClient.RMB = cl

	deployer := NewDeployer(
		tfPluginClient,
		true,
	)

	dl1, err := deploymentWithNameGateway(identity, twinID, true, 0, backendURLWithTLSPassthrough)
	assert.NoError(t, err)
	dl2, err := deploymentWithFQDN(identity, twinID, 0)
	assert.NoError(t, err)

	newDls := map[uint32]gridtypes.Deployment{
		10: dl1,
		20: dl2,
	}

	newDlsData := map[uint32]workloads.DeploymentData{
		10: {},
		20: {},
	}

	newDlsSolProvider := map[uint32]*uint64{
		10: nil,
		20: nil,
	}

	dl1.ContractID = 100
	dl2.ContractID = 200

	dl1Hash, err := hash(&dl1)
	assert.NoError(t, err)
	dl2Hash, err := hash(&dl2)
	assert.NoError(t, err)

	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(10),
			`{"type":"","name":"","projectName":""}`,
			dl1Hash,
			uint32(0),
			nil,
		).Return(uint64(100), nil)

	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(20),
			`{"type":"","name":"","projectName":""}`,
			dl2Hash,
			uint32(0),
			nil,
		).Return(uint64(200), nil)

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(13, cl), nil)

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(20)).
		Return(client.NewNodeClient(23, cl), nil)

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.deploy", dl1, gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			dl1.Workloads[0].Result.State = gridtypes.StateOk
			dl1.Workloads[0].Result.Data, _ = json.Marshal(zos.GatewayProxyResult{})
			return nil
		})

	cl.EXPECT().
		Call(gomock.Any(), uint32(23), "zos.deployment.deploy", dl2, gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			dl2.Workloads[0].Result.State = gridtypes.StateOk
			dl2.Workloads[0].Result.Data, _ = json.Marshal(zos.GatewayFQDNResult{})
			return nil
		})

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl1.Workloads
			return nil
		})

	cl.EXPECT().
		Call(gomock.Any(), uint32(23), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl2.Workloads
			return nil
		})

	deployer.validator = &EmptyValidator{}

	contracts, err := deployer.Deploy(context.Background(), nil, newDls, newDlsData, newDlsSolProvider)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{10: 100, 20: 200})
}

func TestUpdate(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	identity := tfPluginClient.Identity
	twinID := tfPluginClient.TwinID

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	tfPluginClient.SubstrateConn = sub
	tfPluginClient.NcPool = ncPool
	tfPluginClient.RMB = cl

	deployer := NewDeployer(
		tfPluginClient,
		true,
	)

	dl1, err := deploymentWithNameGateway(identity, twinID, true, 0, backendURLWithTLSPassthrough)
	assert.NoError(t, err)
	dl2, err := deploymentWithNameGateway(identity, twinID, true, 1, backendURLWithTLSPassthrough)
	assert.NoError(t, err)

	newDls := map[uint32]gridtypes.Deployment{
		10: dl2,
	}

	newDlsData := map[uint32]workloads.DeploymentData{
		10: {},
	}

	newDlsSolProvider := map[uint32]*uint64{
		10: nil,
	}

	dl1.ContractID = 100
	dl2.ContractID = 100

	dl2Hash, err := hash(&dl2)
	assert.NoError(t, err)

	sub.EXPECT().
		UpdateNodeContract(
			identity,
			uint64(100),
			"",
			dl2Hash,
		).Return(uint64(100), nil)

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(13, cl), nil).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.update", dl2, gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			dl1.Workloads[0].Result.State = gridtypes.StateOk
			dl1.Workloads[0].Result.Data, _ = json.Marshal(zos.GatewayProxyResult{})
			dl1.Version = 1
			dl1.Workloads[0].Version = 1
			return nil
		})

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl1.Workloads
			return nil
		}).AnyTimes()

	deployer.validator = &EmptyValidator{}

	contracts, err := deployer.Deploy(context.Background(), map[uint32]uint64{10: 100}, newDls, newDlsData, newDlsSolProvider)

	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{10: 100})

	assert.Equal(t, dl1.Version, dl2.Version)
	assert.Equal(t, dl1.Workloads[0].Version, dl2.Workloads[0].Version)
}

func TestCancel(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	identity := tfPluginClient.Identity
	twinID := tfPluginClient.TwinID

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	tfPluginClient.SubstrateConn = sub
	tfPluginClient.NcPool = ncPool
	tfPluginClient.RMB = cl

	deployer := NewDeployer(
		tfPluginClient,
		true,
	)

	dl1, err := deploymentWithNameGateway(identity, twinID, true, 0, backendURLWithTLSPassthrough)
	assert.NoError(t, err)

	dl1.ContractID = 100

	sub.EXPECT().
		EnsureContractCanceled(
			identity,
			uint64(100),
		).Return(nil)

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(13, cl), nil).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		})

	deployer.validator = &EmptyValidator{}

	contracts, err := deployer.Deploy(context.Background(), map[uint32]uint64{10: 100}, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{})
}

func TestCocktail(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	identity := tfPluginClient.Identity
	twinID := tfPluginClient.TwinID

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	tfPluginClient.SubstrateConn = sub
	tfPluginClient.NcPool = ncPool
	tfPluginClient.RMB = cl

	deployer := NewDeployer(
		tfPluginClient,
		true,
	)

	g := workloads.GatewayFQDNProxy{Name: "f", FQDN: "test.com", Backends: []zos.Backend{zos.Backend(backendURLWithoutTLSPassthrough)}}

	dl1, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)
	dl2, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)
	dl3, err := deploymentWithNameGateway(identity, twinID, true, 1, backendURLWithTLSPassthrough)
	assert.NoError(t, err)
	dl4, err := deploymentWithNameGateway(identity, twinID, false, 0, backendURLWithoutTLSPassthrough)
	assert.NoError(t, err)
	dl5, err := deploymentWithNameGateway(identity, twinID, true, 0, backendURLWithTLSPassthrough)
	assert.NoError(t, err)
	dl6, err := deploymentWithNameGateway(identity, twinID, true, 0, backendURLWithTLSPassthrough)
	assert.NoError(t, err)

	dl2.Workloads = append(dl2.Workloads, g.ZosWorkload())
	dl3.Workloads = append(dl3.Workloads, g.ZosWorkload())
	assert.NoError(t, dl2.Sign(twinID, identity))
	assert.NoError(t, dl3.Sign(twinID, identity))

	dl1.ContractID = 100
	dl2.ContractID = 200
	dl3.ContractID = 200
	dl4.ContractID = 300

	dl3Hash, err := hash(&dl3)
	assert.NoError(t, err)
	dl4Hash, err := hash(&dl4)
	assert.NoError(t, err)

	oldDls := map[uint32]uint64{
		10: 100,
		20: 200,
		40: 400,
	}
	newDls := map[uint32]gridtypes.Deployment{
		20: dl3,
		30: dl4,
		40: dl6,
	}

	newDlsData := map[uint32]workloads.DeploymentData{
		10: {},
		30: {},
		40: {},
	}

	newDlsSolProvider := map[uint32]*uint64{
		10: nil,
		30: nil,
		40: nil,
	}

	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(30),
			`{"type":"","name":"","projectName":""}`,
			dl4Hash,
			uint32(0),
			nil,
		).Return(uint64(300), nil)

	sub.EXPECT().
		UpdateNodeContract(
			identity,
			uint64(200),
			"",
			dl3Hash,
		).Return(uint64(200), nil)

	sub.EXPECT().
		EnsureContractCanceled(
			identity,
			uint64(100),
		).Return(nil)

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(13, cl), nil).AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(20)).
		Return(client.NewNodeClient(23, cl), nil).AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(30)).
		Return(client.NewNodeClient(33, cl), nil).AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(40)).
		Return(client.NewNodeClient(43, cl), nil).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl1.Workloads
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(23), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl2.Workloads
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(33), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl4.Workloads
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(43), "zos.deployment.changes", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *[]gridtypes.Workload = result.(*[]gridtypes.Workload)
			*res = dl5.Workloads
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(13), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl1
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(23), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl2
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(33), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl4
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(43), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = dl5
			return nil
		}).AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), uint32(23), "zos.deployment.update", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			dl2.Workloads = dl3.Workloads
			dl2.Version = 1
			dl2.Workloads[0].Version = 1
			dl2.Workloads[0].Result.State = gridtypes.StateOk
			dl2.Workloads[0].Result.Data, _ = json.Marshal(zos.GatewayProxyResult{})
			dl2.Workloads[1].Result.State = gridtypes.StateOk
			dl2.Workloads[1].Result.Data, _ = json.Marshal(zos.GatewayProxyResult{})
			return nil
		})

	cl.EXPECT().
		Call(gomock.Any(), uint32(33), "zos.deployment.deploy", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			dl4.Workloads[0].Result.State = gridtypes.StateOk
			dl4.Workloads[0].Result.Data, _ = json.Marshal(zos.GatewayProxyResult{})
			return nil
		})

	deployer.validator = &EmptyValidator{}

	contracts, err := deployer.Deploy(context.Background(), oldDls, newDls, newDlsData, newDlsSolProvider)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{
		20: 200,
		30: 300,
		40: 400,
	})
}
