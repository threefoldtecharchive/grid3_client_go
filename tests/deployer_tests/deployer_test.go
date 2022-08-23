package deployertests

import (
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	mock "github.com/threefoldtech/grid3-go/tests/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const Words = "secret add bag cluster deposit beach illness letter crouch position rain arctic"
const twinID = 214

var identity, _ = substrate.NewIdentityFromEd25519Phrase(Words)

func deployment1(identity substrate.Identity, TLSPassthrough bool, version uint32) gridtypes.Deployment {
	dl := workloads.NewDeployment(twinID)
	dl.Version = version

	gw := workloads.GatewayNameProxy{
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
	dl := workloads.NewDeployment(uint32(twinID))
	gw := workloads.GatewayFQDNProxy{
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

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	gridClient := mock.NewMockClient(ctrl)
	// cl := mock.

}
