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

func constructTestNetworkDeployer(t *testing.T, mock bool) (NetworkDeployer, *mocks.RMBMockClient, *mocks.MockSubstrateExt, *mocks.MockNodeClientGetter) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	if mock {
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl

		tfPluginClient.StateLoader.ncPool = ncPool
		tfPluginClient.StateLoader.substrate = sub

		tfPluginClient.NetworkDeployer.tfPluginClient = &tfPluginClient
	}

	return tfPluginClient.NetworkDeployer, cl, sub, ncPool
}

func TestNetworkValidate(t *testing.T) {
	znet := constructTestNetwork()
	znet.Nodes = []uint32{}
	d, _, _, _ := constructTestNetworkDeployer(t, false)

	znet.IPRange.Mask = net.CIDRMask(20, 32)
	assert.Error(t, d.Validate(context.Background(), &znet))

	znet.IPRange.Mask = net.CIDRMask(16, 32)
	assert.NoError(t, d.Validate(context.Background(), &znet))
}

func TestNetworkGenerateDeployment(t *testing.T) {
	net := constructTestNetwork()
	d, cl, sub, ncPool := constructTestNetworkDeployer(t, true)

	d.tfPluginClient.StateLoader.currentNodeNetwork[nodeID] = contractID

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
		Return(client.NewNodeClient(twinID, cl), nil).
		AnyTimes()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &net)
	assert.NoError(t, err)

	workload := net.ZosWorkload(net.NodesIPRange[nodeID], d.Keys[nodeID].String(), uint16(d.WGPort[nodeID]), []zos.Peer{})
	networkDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{workload})

	assert.Equal(t, len(networkDl.Workloads), len(dls[net.Nodes[0]].Workloads))
	assert.Equal(t, networkDl.Workloads, dls[net.Nodes[0]].Workloads)
}
