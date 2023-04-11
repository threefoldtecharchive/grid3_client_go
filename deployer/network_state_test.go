// Package deployer for grid deployer
package deployer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkState(t *testing.T) {
	net := constructTestNetwork()
	nodeID = net.Nodes[0]

	networkState := NetworkState{net.Name: Network{
		Subnets:               map[uint32]string{nodeID: net.IPRange.String()},
		NodeDeploymentHostIDs: map[uint32]DeploymentHostIDs{nodeID: map[uint64][]byte{contractID: {}}},
	}}
	network := networkState.GetNetwork(net.Name)

	assert.Equal(t, network.getNodeSubnet(nodeID), net.IPRange.String())
	assert.Empty(t, network.GetDeploymentHostIDs(nodeID, contractID))

	network.SetNodeSubnet(nodeID, "10.1.1.0/24")
	assert.Equal(t, network.getNodeSubnet(nodeID), "10.1.1.0/24")

	network.SetDeploymentHostIDs(nodeID, contractID, []byte{1, 2, 3})
	assert.Equal(t, network.GetDeploymentHostIDs(nodeID, contractID), []byte{1, 2, 3})

	network.deleteNodeSubnet(nodeID)
	assert.Empty(t, network.getNodeSubnet(nodeID))

	network.DeleteDeploymentHostIDs(nodeID, contractID)
	assert.Empty(t, network.GetDeploymentHostIDs(nodeID, contractID))
}
