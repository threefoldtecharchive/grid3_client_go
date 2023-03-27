// Package deployer is the grid deployer
package deployer

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func constructTestNetwork() workloads.ZNet {
	return workloads.ZNet{
		Name:        "network",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
}

func constructTestNetworkDeployer(t *testing.T, tfPluginClient TFPluginClient, mock bool) (NetworkDeployer, *mocks.RMBMockClient, *mocks.MockSubstrateExt, *mocks.MockNodeClientGetter) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	if mock {
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl

		tfPluginClient.State.ncPool = ncPool
		tfPluginClient.State.substrate = sub

		tfPluginClient.twinID = twinID

		tfPluginClient.NetworkDeployer.tfPluginClient = &tfPluginClient
	}

	return tfPluginClient.NetworkDeployer, cl, sub, ncPool
}

func TestNetworkDeployer(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	t.Run("test validate", func(t *testing.T) {
		znet := constructTestNetwork()
		znet.Nodes = []uint32{}
		d, _, _, _ := constructTestNetworkDeployer(t, tfPluginClient, false)

		znet.IPRange.Mask = net.CIDRMask(20, 32)
		assert.Error(t, d.Validate(context.Background(), &znet))

		znet.IPRange.Mask = net.CIDRMask(16, 32)
		assert.NoError(t, d.Validate(context.Background(), &znet))
	})

	d, cl, sub, ncPool := constructTestNetworkDeployer(t, tfPluginClient, true)
	znet := constructTestNetwork()

	t.Run("test generate", func(t *testing.T) {
		znet.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

		cl.EXPECT().
			Call(gomock.Any(), twinID, "zos.network.public_config_get", gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		cl.EXPECT().
			Call(gomock.Any(), twinID, "zos.network.interfaces", gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		cl.EXPECT().
			Call(gomock.Any(), twinID, "zos.network.list_wg_ports", gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		ncPool.EXPECT().
			GetNodeClient(sub, uint32(nodeID)).
			Return(client.NewNodeClient(twinID, cl, d.tfPluginClient.rmbTimeout), nil).
			AnyTimes()

		dls, err := d.GenerateVersionlessDeployments(context.Background(), &znet)
		assert.NoError(t, err)

		workload := znet.ZosWorkload(znet.NodesIPRange[nodeID], znet.Keys[nodeID].String(), uint16(znet.WGPort[nodeID]), []zos.Peer{})
		networkDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{workload})

		networkDl.Metadata = "{\"type\":\"network\",\"name\":\"network\",\"projectName\":\"Network\"}"

		assert.Equal(t, len(networkDl.Workloads), len(dls[znet.Nodes[0]].Workloads))
		assert.Equal(t, networkDl.Workloads, dls[znet.Nodes[0]].Workloads)
		assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
			nodeID: networkDl,
		})
	})
}
