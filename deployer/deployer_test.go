package deployer

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	mock "github.com/threefoldtech/grid3-go/tests/mocks"

	"github.com/threefoldtech/substrate-client"

	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const Words = "logic bag student thing good immune hood clip alley pigeon color wedding"
const twinID = 280

var identity, _ = substrate.NewIdentityFromEd25519Phrase(Words)

func deployment1(identity substrate.Identity, TLSPassthrough bool, version uint32) gridtypes.Deployment {
	dl := NewDeployment(uint32(twinID))
	dl.Version = version
	gw := GatewayNameProxy{
		Name:           "name",
		TLSPassthrough: TLSPassthrough,
		Backends:       []zos.Backend{"http://1.1.1.1"},
	}

	workload, err := gw.GenerateWorkloadFromGName(gw)
	if err != nil {
		panic(err)
	}
	dl.Workloads = append(dl.Workloads, workload)
	dl.Workloads[0].Version = version
	err = dl.Sign(twinID, identity)
	if err != nil {
		panic(err)
	}
	return dl
}

func deployment2(identity substrate.Identity) gridtypes.Deployment {
	dl := NewDeployment(uint32(twinID))
	gw := GatewayFQDNProxy{
		Name:     "fqdn",
		FQDN:     "a.b.com",
		Backends: []zos.Backend{"http://1.1.1.1"},
	}

	workload, err := gw.GenerateWorkloadFromFQDN(gw)
	if err != nil {
		panic(err)
	}
	dl.Workloads = append(dl.Workloads, workload)
	err = dl.Sign(twinID, identity)
	if err != nil {
		panic(err)
	}

	return dl
}
func hash(dl *gridtypes.Deployment) string {
	hash, err := dl.ChallengeHash()
	if err != nil {
		panic(err)
	}
	hashHex := hex.EncodeToString(hash)
	return hashHex
}

type EmptyValidator struct{}

func (d *EmptyValidator) Validate(ctx context.Context, sub subi.SubstrateExt, oldDeployments map[uint32]gridtypes.Deployment, newDeployments map[uint32]gridtypes.Deployment) error {
	return nil
}
func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	gridClient := mock.NewMockClient(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	ncPool := mock.NewMockNodeClientCollection(ctrl)
	newDeployer := NewDeployer(
		identity,
		280,
		gridClient,
		ncPool,
		true,
	)
	dl1, dl2 := deployment1(identity, true, 0), deployment2(identity)
	newDls := map[uint32]gridtypes.Deployment{
		10: dl1,
		20: dl2,
	}
	dl1.ContractID = 100
	dl2.ContractID = 200
	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(10),
			"",
			hash(&dl1),
			uint32(0),
			nil,
		).Return(uint64(100), nil)
	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(20),
			"",
			hash(&dl2),
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
	newDeployer.(*DeployerImpl).validator = &EmptyValidator{}
	contracts, err := newDeployer.Deploy(context.Background(), sub, nil, newDls)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{10: 100, 20: 200})
}

func TestUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	gridClient := mock.NewMockClient(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	ncPool := mock.NewMockNodeClientCollection(ctrl)
	newDeployer := NewDeployer(
		identity,
		280,
		gridClient,
		ncPool,
		true,
	)
	dl1, dl2 := deployment1(identity, false, 0), deployment1(identity, true, 1)
	newDls := map[uint32]gridtypes.Deployment{
		10: dl2,
	}

	dl1.ContractID = 100
	dl2.ContractID = 100
	sub.EXPECT().
		UpdateNodeContract(
			identity,
			uint64(100),
			"",
			hash(&dl2),
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
	newDeployer.(*DeployerImpl).validator = &EmptyValidator{}
	contracts, err := newDeployer.Deploy(context.Background(), sub, map[uint32]uint64{10: 100}, newDls)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{10: 100})
	assert.Equal(t, dl1.Version, dl2.Version)
	assert.Equal(t, dl1.Workloads[0].Version, dl2.Workloads[0].Version)
}

func TestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	gridClient := mock.NewMockClient(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	ncPool := mock.NewMockNodeClientCollection(ctrl)
	newDeployer := NewDeployer(
		identity,
		280,
		gridClient,
		ncPool,
		true,
	)
	dl1 := deployment1(identity, false, 0)
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
	newDeployer.(*DeployerImpl).validator = &EmptyValidator{}
	contracts, err := newDeployer.Deploy(context.Background(), sub, map[uint32]uint64{10: 100}, nil)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{})
}

func TestCocktail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	gridClient := mock.NewMockClient(ctrl)
	cl := mock.NewRMBMockClient(ctrl)
	sub := mock.NewMockSubstrateExt(ctrl)
	ncPool := mock.NewMockNodeClientCollection(ctrl)
	newDeployer := NewDeployer(
		identity,
		11,
		gridClient,
		ncPool,
		true,
	)
	g := GatewayFQDNProxy{Name: "f", FQDN: "test.com", Backends: []zos.Backend{"http://1.1.1.1:10"}}
	workload, err := g.GenerateWorkloadFromFQDN(g)
	if err != nil {
		panic(err)
	}
	dl1 := deployment1(identity, false, 0)
	dl2, dl3 := deployment1(identity, false, 0), deployment1(identity, true, 1)
	dl5, dl6 := deployment1(identity, true, 0), deployment1(identity, true, 0)
	dl2.Workloads = append(dl2.Workloads, workload)
	dl3.Workloads = append(dl3.Workloads, workload)
	assert.NoError(t, dl2.Sign(twinID, identity))
	assert.NoError(t, dl3.Sign(twinID, identity))
	dl4 := deployment1(identity, false, 0)
	dl1.ContractID = 100
	dl2.ContractID = 200
	dl3.ContractID = 200
	dl4.ContractID = 300
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
	sub.EXPECT().
		CreateNodeContract(
			identity,
			uint32(30),
			"",
			hash(&dl4),
			uint32(0),
			nil,
		).Return(uint64(300), nil)

	sub.EXPECT().
		UpdateNodeContract(
			identity,
			uint64(200),
			"",
			hash(&dl3),
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
	newDeployer.(*DeployerImpl).validator = &EmptyValidator{}
	contracts, err := newDeployer.Deploy(context.Background(), sub, oldDls, newDls)
	assert.NoError(t, err)
	assert.Equal(t, contracts, map[uint32]uint64{
		20: 200,
		30: 300,
		40: 400,
	})
}

func NewDeployment(twin uint32) gridtypes.Deployment {
	return gridtypes.Deployment{
		Version: 0,
		TwinID:  twin, //LocalTwin,
		// this contract id must match the one on substrate
		Workloads: []gridtypes.Workload{},
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: twin,
					Weight: 1,
				},
			},
		},
	}
}

type GatewayNameProxy struct {
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string

	// Passthrough whether to pass tls traffic or not
	TLSPassthrough bool

	// Backends are list of backend ips
	Backends []zos.Backend

	// FQDN deployed on the node
	FQDN string
}

func (g *GatewayNameProxy) GenerateWorkloadFromGName(gatewayName GatewayNameProxy) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(gatewayName.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           gatewayName.Name,
			TLSPassthrough: gatewayName.TLSPassthrough,
			Backends:       gatewayName.Backends,
		}),
	}, nil
}

type GatewayFQDNProxy struct {
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string

	// Passthrough whether to pass tls traffic or not
	TLSPassthrough bool

	// Backends are list of backend ips
	Backends []zos.Backend

	// FQDN deployed on the node
	FQDN string
}

func (g *GatewayFQDNProxy) GenerateWorkloadFromFQDN(gatewayFQDN GatewayFQDNProxy) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayFQDNProxyType,
		Name:    gridtypes.Name(gatewayFQDN.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			TLSPassthrough: gatewayFQDN.TLSPassthrough,
			Backends:       gatewayFQDN.Backends,
			FQDN:           gatewayFQDN.FQDN,
		}),
	}, nil
}
