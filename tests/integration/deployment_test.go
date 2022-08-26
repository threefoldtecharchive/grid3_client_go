package integration

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestDeployment(t *testing.T) {
	identity, err := subi.NewIdentityFromSr25519Phrase("route visual hundred rabbit wet crunch ice castle milk model inherit outside")
	assert.Equal(t, nil, err)
	sk, err := identity.KeyPair()
	assert.Equal(t, nil, err)
	sub := subi.NewManager("wss://tfchain.dev.grid.tf/ws")
	pub := sk.Public()
	subext, err := sub.SubstrateExt()
	assert.Equal(t, nil, err)
	defer subext.Close()
	twin, err := subext.GetTwinByPubKey(pub)
	assert.Equal(t, nil, err)
	cl, err := client.NewProxyBus("https://gridproxy.dev.grid.tf/", twin, sub, identity, true)
	assert.Equal(t, nil, err)
	gridClient := proxy.NewRetryingClient(proxy.NewClient("https://gridproxy.dev.grid.tf/"))
	ncPool := client.NewNodeClientPool(cl)
	manager := deployer.NewDeploymentManager(identity, twin, map[uint32]uint64{}, gridClient, ncPool, sub)
	apiClient := workloads.APIClient{
		SubstrateExt: subext,
		NCPool:       ncPool,
		ProxyClient:  gridClient,
		Manager:      manager,
		Identity:     identity,
	}

	// build deployment
	userAccess := &workloads.UserAccess{}
	log.Printf("useraccess: %+v", userAccess)
	network := workloads.TargetNetwork{
		Name:        "skynet1",
		Description: "not skynet",
		Nodes:       []uint32{33, 34},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
	// vm := workloads.VM{
	// 	Name: "vm1",
	// }

	err = network.Stage(context.Background(), apiClient, userAccess)
	assert.Equal(t, nil, err)

	log.Printf("user access after staging: %v", *userAccess)
	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)
	log.Printf("current contracts: %+v", manager.GetDeployments())

	log.Printf("modifying network")
	network.AddWGAccess = true
	network.Nodes = []uint32{33, 31}
	err = network.Stage(context.Background(), apiClient, userAccess)
	assert.Equal(t, nil, err)

	log.Printf("user access after staging: %v", *userAccess)
	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	log.Printf("current contracts: %+v", manager.GetDeployments())
	err = manager.CancelAll()
	log.Printf("current contracts: %+v", manager.GetDeployments())
}
