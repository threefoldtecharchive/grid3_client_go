// Package deployer for grid deployer
package deployer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkState(t *testing.T) {
	net := constructTestNetwork()
	nodeID = net.Nodes[0]

	networkState := networkState{net.Name: network{
		subnets:               map[uint32]string{nodeID: net.IPRange.String()},
		nodeDeploymentHostIDs: map[uint32]deploymentHostIDs{nodeID: map[uint64][]byte{contractID: {}}},
	}}
	network := networkState.getNetwork(net.Name)

	assert.Equal(t, network.getNodeSubnet(nodeID), net.IPRange.String())
	assert.Empty(t, network.getDeploymentHostIDs(nodeID, contractID))

	network.setNodeSubnet(nodeID, "10.1.1.0/24")
	assert.Equal(t, network.getNodeSubnet(nodeID), "10.1.1.0/24")

	network.setDeploymentHostIDs(nodeID, contractID, []byte{1, 2, 3})
	assert.Equal(t, network.getDeploymentHostIDs(nodeID, contractID), []byte{1, 2, 3})

	network.deleteNodeSubnet(nodeID)
	assert.Empty(t, network.getNodeSubnet(nodeID))

	network.deleteDeploymentHostIDs(nodeID, contractID)
	assert.Empty(t, network.getDeploymentHostIDs(nodeID, contractID))
}
