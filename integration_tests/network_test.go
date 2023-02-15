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

	filter := deployer.NodeFilter{
		CRU:    2,
		SRU:    2,
		MRU:    1,
		Status: "up",
	}
	nodeIDs, err := deployer.FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)
	nodeIDs, err = deployer.FilterNodesWithPublicConfigs(tfPluginClient.SubstrateConn, tfPluginClient.NcPool, nodeIDs)
	assert.NoError(t, err)

	network := workloads.ZNet{
		Name:        "net1",
		Description: "not skynet",
		Nodes:       nodeIDs[:1],
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
		networkCp.Nodes = []uint32{nodeIDs[1]}

		err = tfPluginClient.NetworkDeployer.Deploy(ctx, &networkCp)
		assert.NoError(t, err)

		_, err := tfPluginClient.State.LoadNetworkFromGrid(networkCp.Name)
		assert.NoError(t, err)
	})

	t.Run("update network remove wireguard access", func(t *testing.T) {
		networkCp.AddWGAccess = false
		networkCp.Nodes = []uint32{nodeIDs[1]}

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

}
