// Package integration for integration tests
package integration

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestDeployment(t *testing.T) {

	manager, apiClient := setup()

	network := workloads.ZNet{
		Name:        "net1",
		Description: "not skynet",
		Nodes:       []uint32{33, 34},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: true,
	}

	networkManager, err := deployer.NewNetworkDeployer(apiClient.Manager, network)
	assert.NoError(t, err)

	// vm := workloads.VM{
	// 	Name: "vm1",
	// }

	access, err := networkManager.Stage(context.Background(), apiClient, network)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	err = manager.CancelAll()
	assert.NoError(t, err)

	ln, err := loader.LoadNetworkFromGrid(manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	// network.AddWGAccess = true
	network.Nodes = []uint32{33, 31}
	access, err = networkManager.Stage(context.Background(), apiClient, network)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	network.AddWGAccess = false
	network.Nodes = []uint32{33, 31}
	access, err = networkManager.Stage(context.Background(), apiClient, network)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.NoError(t, err)
	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	err = manager.CancelAll()
	assert.NoError(t, err)

	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	assert.NoError(t, err)

	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())
}
