// Package deployer for grid deployer
package deployer

import "github.com/threefoldtech/zos/pkg/gridtypes"

// NetworkState is a map of of names and their networks
type NetworkState map[string]Network

// Network struct includes Subnets and node IPs
type Network struct {
	Subnets               map[uint32]string
	NodeDeploymentHostIDs NodeDeploymentHostIDs
}

// NodeDeploymentHostIDs is a map for nodes ID and its deployments' IPs
type NodeDeploymentHostIDs map[uint32]DeploymentHostIDs

// DeploymentHostIDs is a map for deployment and its IPs
type DeploymentHostIDs map[uint64][]byte

// NewNetwork creates a new Network
func NewNetwork() Network {
	return Network{
		Subnets:               map[uint32]string{},
		NodeDeploymentHostIDs: NodeDeploymentHostIDs{},
	}
}

// GetNetwork get a Network using its name
func (nm NetworkState) GetNetwork(networkName string) Network {
	if _, ok := nm[networkName]; !ok {
		nm[networkName] = NewNetwork()
	}
	net := nm[networkName]
	return net
}

// UpdateNetwork updates a network subnets given its name
func (nm NetworkState) UpdateNetwork(networkName string, ipRange map[uint32]gridtypes.IPNet) {
	nm.DeleteNetwork(networkName)
	Network := nm.GetNetwork(networkName)
	for nodeID, subnet := range ipRange {
		Network.SetNodeSubnet(nodeID, subnet.String())
	}
}

// DeleteNetwork deletes a Network using its name
func (nm NetworkState) DeleteNetwork(networkName string) {
	delete(nm, networkName)
}

// GetNodeSubnet gets a node subnet using its ID
func (n *Network) getNodeSubnet(nodeID uint32) string {
	return n.Subnets[nodeID]
}

// SetNodeSubnet sets a node subnet with its ID and subnet
func (n *Network) SetNodeSubnet(nodeID uint32, subnet string) {
	n.Subnets[nodeID] = subnet
}

// DeleteNodeSubnet deletes a node subnet using its ID
func (n *Network) deleteNodeSubnet(nodeID uint32) {
	delete(n.Subnets, nodeID)
}

// GetUsedNetworkHostIDs gets the used host IDs on the overlay Network
func (n *Network) getUsedNetworkHostIDs(nodeID uint32) []byte {
	ips := []byte{}
	for _, v := range n.NodeDeploymentHostIDs[nodeID] {
		ips = append(ips, v...)
	}
	return ips
}

// GetDeploymentHostIDs gets the private Network host IDs relevant to the deployment
func (n *Network) GetDeploymentHostIDs(nodeID uint32, contractID uint64) []byte {
	if n.NodeDeploymentHostIDs[nodeID] == nil {
		return []byte{}
	}
	return n.NodeDeploymentHostIDs[nodeID][contractID]
}

// SetDeploymentHostIDs sets the relevant deployment host IDs
func (n *Network) SetDeploymentHostIDs(nodeID uint32, contractID uint64, ips []byte) {
	if n.NodeDeploymentHostIDs[nodeID] == nil {
		n.NodeDeploymentHostIDs[nodeID] = DeploymentHostIDs{}
	}
	n.NodeDeploymentHostIDs[nodeID][contractID] = ips
}

// DeleteDeploymentHostIDs deletes a deployment host IDs
func (n *Network) DeleteDeploymentHostIDs(nodeID uint32, contractID uint64) {
	delete(n.NodeDeploymentHostIDs[nodeID], contractID)
}
