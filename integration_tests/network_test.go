// Package integration for integration tests
package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestNetworkDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	nodes, err := deployer.FilterNodes(tfPluginClient.GridProxyClient, nodeFilter)
	assert.NoError(t, err)

	nodeID1 := uint32(nodes[0].NodeID)
	nodeID2 := uint32(nodes[1].NodeID)

	network := workloads.ZNet{
		Name:        "net1",
		Description: "not skynet",
		Nodes:       []uint32{nodeID1},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: true,
	}

	networkCp := network

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Run("deploy network with wireguard access", func(t *testing.T) {
		err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
		assert.NoError(t, err)

		_, err := tfPluginClient.State.LoadNetworkFromGrid(network.Name)
		assert.NoError(t, err)
	})

	t.Run("deploy network with wireguard access on different nodes", func(t *testing.T) {
		networkCp.Nodes = []uint32{nodeID2}

		err = tfPluginClient.NetworkDeployer.Deploy(ctx, &networkCp)
		assert.NoError(t, err)

		_, err := tfPluginClient.State.LoadNetworkFromGrid(networkCp.Name)
		assert.NoError(t, err)
	})

	t.Run("update network remove wireguard access", func(t *testing.T) {
		networkCp.AddWGAccess = false
		networkCp.Nodes = []uint32{nodeID2}

		err = tfPluginClient.NetworkDeployer.Deploy(ctx, &networkCp)
		assert.NoError(t, err)

		_, err := tfPluginClient.State.LoadNetworkFromGrid(networkCp.Name)
		assert.NoError(t, err)
	})

	t.Run("cancel network", func(t *testing.T) {
		err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
		assert.NoError(t, err)

		err = tfPluginClient.NetworkDeployer.Cancel(ctx, &networkCp)
		assert.NoError(t, err)

		_, err := tfPluginClient.State.LoadNetworkFromGrid(network.Name)
		assert.Error(t, err)
	})

	t.Run("test get public node", func(t *testing.T) {
		publicNodeID, err := deployer.GetPublicNode(
			context.Background(),
			tfPluginClient.GridProxyClient,
			[]uint32{},
		)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, publicNodeID)
	})
}
