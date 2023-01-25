// Package deployer for grid deployer
package deployer

import "github.com/threefoldtech/zos/pkg/gridtypes"

// networkState is a map of of names and their networks
type networkState map[string]network

// network struct includes subnets and node IPs
type network struct {
	subnets               map[uint32]string     `json:"subnets"`
	nodeDeploymentHostIDs nodeDeploymentHostIDs `json:"node_ips"`
}

// nodeDeploymentHostIDs is a map for nodes ID and its deployments' IPs
type nodeDeploymentHostIDs map[uint32]deploymentHostIDs

// deploymentHostIDs is a map for deployment and its IPs
type deploymentHostIDs map[uint64][]byte

// NewNetwork creates a new Network
func newNetwork() network {
	return network{
		subnets:               map[uint32]string{},
		nodeDeploymentHostIDs: nodeDeploymentHostIDs{},
	}
}

// GetNetwork get a network using its name
func (nm networkState) getNetwork(networkName string) network {
	if _, ok := nm[networkName]; !ok {
		nm[networkName] = newNetwork()
	}
	net := nm[networkName]
	return net
}

func (nm networkState) updateNetwork(networkName string, ipRange map[uint32]gridtypes.IPNet) {
	nm.deleteNetwork(networkName)
	network := nm.getNetwork(networkName)
	for nodeID, subnet := range ipRange {
		network.setNodeSubnet(nodeID, subnet.String())
	}
}

// DeleteNetwork deletes a network using its name
func (nm networkState) deleteNetwork(networkName string) {
	delete(nm, networkName)
}

// GetNodeSubnet gets a node subnet using its ID
func (n *network) getNodeSubnet(nodeID uint32) string {
	return n.subnets[nodeID]
}

// SetNodeSubnet sets a node subnet with its ID and subnet
func (n *network) setNodeSubnet(nodeID uint32, subnet string) {
	n.subnets[nodeID] = subnet
}

// DeleteNodeSubnet deletes a node subnet using its ID
func (n *network) deleteNodeSubnet(nodeID uint32) {
	delete(n.subnets, nodeID)
}

// GetUsedNetworkHostIDs gets the used host IDs on the overlay network
func (n *network) getUsedNetworkHostIDs(nodeID uint32) []byte {
	ips := []byte{}
	for _, v := range n.nodeDeploymentHostIDs[nodeID] {
		ips = append(ips, v...)
	}
	return ips
}

// GetDeploymentHostIDs gets the private network host IDs relevant to the deployment
func (n *network) getDeploymentHostIDs(nodeID uint32, contractID uint64) []byte {
	if n.nodeDeploymentHostIDs[nodeID] == nil {
		return []byte{}
	}
	return n.nodeDeploymentHostIDs[nodeID][contractID]
}

// SetDeploymentHostIDs sets the relevant deployment host IDs
func (n *network) setDeploymentHostIDs(nodeID uint32, contractID uint64, ips []byte) {
	if n.nodeDeploymentHostIDs[nodeID] == nil {
		n.nodeDeploymentHostIDs[nodeID] = deploymentHostIDs{}
	}
	n.nodeDeploymentHostIDs[nodeID][contractID] = ips
}

// DeleteDeploymentHostIDs deletes a deployment host IDs
func (n *network) deleteDeploymentHostIDs(nodeID uint32, contractID uint64) {
	delete(n.nodeDeploymentHostIDs[nodeID], contractID)
}
