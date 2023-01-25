// Package integration for integration tests
package integration

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/manager"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	networkManager, err := manager.NewNetworkDeployer(ctx, network, "", &tfPluginClient)
	assert.NoError(t, err)

	// vm := workloads.VM{
	// 	Name: "vm1",
	// }

	err = networkManager.Stage(ctx)
	assert.Equal(t, nil, err)

	err = tfPluginClient.Manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)

	ln, err := manager.LoadNetworkFromGrid(tfPluginClient.Manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", tfPluginClient.Manager.GetContractIDs())

	// network.AddWGAccess = true
	network.Nodes = []uint32{33, 31}
	err = networkManager.Stage(ctx)
	assert.Equal(t, nil, err)

	err = tfPluginClient.Manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	ln, err = manager.LoadNetworkFromGrid(tfPluginClient.Manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", tfPluginClient.Manager.GetContractIDs())

	network.AddWGAccess = false
	network.Nodes = []uint32{33, 31}
	err = networkManager.Stage(ctx)
	assert.Equal(t, nil, err)

	err = tfPluginClient.Manager.Commit(context.Background())
	assert.NoError(t, err)
	ln, err = manager.LoadNetworkFromGrid(tfPluginClient.Manager, "net1")
	assert.NoError(t, err)
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", tfPluginClient.Manager.GetContractIDs())

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)

	ln, err = manager.LoadNetworkFromGrid(tfPluginClient.Manager, "net1")
	assert.NoError(t, err)

	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", tfPluginClient.Manager.GetContractIDs())
}
