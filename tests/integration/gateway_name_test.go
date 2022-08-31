package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	// "github.com/stretchr/testify/assert"
	// "github.com/threefoldtech/grid3-go/loader"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const words = "secret add bag cluster deposit beach illness letter crouch position rain arctic"
const twinId = 214

var identity, err = substrate.NewIdentityFromEd25519Phrase(words)

func TestGatewayNameDeployment(t *testing.T) {
	assert.NoError(t, err)
	subManager := subi.NewManager("wss://tfchain.dev.grid.tf/ws")
	cl, err := client.NewProxyBus("https://gridproxy.dev.grid.tf/", twinId, subManager, identity, true)
	assert.NoError(t, err)
	// fmt.Println(cl)
	manager := deployer.NewDeploymentManager(identity, twinId, map[uint32]uint64{}, proxy.NewClient("https://gridproxy.dev.grid.tf/"), client.NewNodeClientPool(cl), subManager)
	fmt.Println(manager)
	backend := "http://185.206.122.36/24"
	expected := workloads.GatewayNameProxy{
		Name:           "TestGatewayName",
		TLSPassthrough: true,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "fqdn",
	}
	// fmt.Println(expected)

	err = expected.Stage(manager, 11)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	// result, err := loader.LoadGatewayNameFromGrid(manager, 11, "TestGatewayName")
	// assert.NoError(t, err)

	// assert.NotEmpty(t, result.Backends)
	// assert.NotEmpty(t, result.FQDN)
	// assert.NotEmpty(t, result.Name)
	// assert.NotEmpty(t, result.TLSPassthrough)

	// result.Backends = nil
	// result.FQDN = ""
	// result.Name = ""
	// result.TLSPassthrough = false
	// assert.Equal(t, expected, result)
	// err = manager.CancelAll()
	// assert.NoError(t, err)
	// _, err = loader.LoadGatewayNameFromGrid(manager, 11, "testName")
	// assert.Error(t, err)

}
