// Package deployer for grid deployer
package deployer

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// NetworkDeployer struct
type NetworkDeployer struct {
	WGPort map[uint32]int
	Keys   map[uint32]wgtypes.Key

	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(tfPluginClient *TFPluginClient) NetworkDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	return NetworkDeployer{
		Keys:           make(map[uint32]wgtypes.Key),
		WGPort:         make(map[uint32]int),
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}
}

// Validate validates a network deployer
func (d *NetworkDeployer) Validate(ctx context.Context, znet *workloads.ZNet) error {
	sub := d.tfPluginClient.SubstrateConn

	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.identity); err != nil {
		return err
	}

	if err := znet.Validate(); err != nil {
		return err
	}

	err := client.AreNodesUp(ctx, sub, znet.Nodes, d.tfPluginClient.NcPool)
	if err != nil {
		return err
	}

	return d.invalidateBrokenAttributes(znet)
}

// GenerateVersionlessDeployments generates deployments for network deployer without versions
func (d *NetworkDeployer) GenerateVersionlessDeployments(ctx context.Context, znet *workloads.ZNet) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)

	log.Printf("nodes: %v\n", znet.Nodes)
	sub := d.tfPluginClient.SubstrateConn

	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	accessibleNodes := make([]uint32, 0)
	var ipv4Node uint32

	for _, nodeID := range znet.Nodes {
		nodeClient, err := d.tfPluginClient.NcPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't get node %d client", nodeID)
		}

		endpoint, err := workloads.GetNodeEndpoint(ctx, nodeClient)
		if errors.Is(err, workloads.ErrNoAccessibleInterfaceFound) {
			hiddenNodes = append(hiddenNodes, nodeID)
		} else if err != nil {
			return nil, errors.Wrapf(err, "failed to get node %d endpoint", nodeID)
		} else if endpoint.To4() != nil {
			accessibleNodes = append(accessibleNodes, nodeID)
			ipv4Node = nodeID
			endpoints[nodeID] = endpoint.String()
		} else {
			accessibleNodes = append(accessibleNodes, nodeID)
			endpoints[nodeID] = fmt.Sprintf("[%s]", endpoint.String())
		}
	}

	needsIPv4Access := znet.AddWGAccess || (len(hiddenNodes) != 0 && len(hiddenNodes)+len(accessibleNodes) > 1)
	if needsIPv4Access {
		if znet.PublicNodeID != 0 { // it's set
			// if public node id is already set, it should be added to accessible nodes
			if !workloads.Contains(accessibleNodes, znet.PublicNodeID) {
				accessibleNodes = append(accessibleNodes, znet.PublicNodeID)
			}
		} else if ipv4Node != 0 { // there's one in the network original nodes
			znet.PublicNodeID = ipv4Node
		} else {
			publicNode, err := workloads.GetPublicNode(ctx, d.tfPluginClient.GridProxyClient, []uint32{})
			if err != nil {
				return nil, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			znet.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}

		if endpoints[znet.PublicNodeID] == "" { // old or new outsider
			cl, err := d.tfPluginClient.NcPool.GetNodeClient(sub, znet.PublicNodeID)
			if err != nil {
				return nil, errors.Wrapf(err, "couldn't get node %d client", znet.PublicNodeID)
			}
			endpoint, err := workloads.GetNodeEndpoint(ctx, cl)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get node %d endpoint", znet.PublicNodeID)
			}
			endpoints[znet.PublicNodeID] = endpoint.String()
		}
	}

	allNodes := append(hiddenNodes, accessibleNodes...)
	if err := d.assignNodesIPs(allNodes, znet); err != nil {
		return nil, errors.Wrap(err, "couldn't assign node ips")
	}
	if err := d.assignNodesWGKey(allNodes); err != nil {
		return nil, errors.Wrap(err, "couldn't assign node wg keys")
	}
	if err := d.assignNodesWGPort(ctx, sub, allNodes); err != nil {
		return nil, errors.Wrap(err, "couldn't assign node wg ports")
	}

	nonAccessibleIPRanges := []gridtypes.IPNet{}
	for _, nodeID := range hiddenNodes {
		r := znet.NodesIPRange[nodeID]
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(r))
	}
	if znet.AddWGAccess {
		r := znet.ExternalIP
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, *r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(*r))
	}

	log.Printf("hidden nodes: %v\n", hiddenNodes)
	log.Printf("public node: %v\n", znet.PublicNodeID)
	log.Printf("accessible nodes: %v\n", accessibleNodes)
	log.Printf("non accessible ip ranges: %v\n", nonAccessibleIPRanges)

	if znet.AddWGAccess {
		znet.AccessWGConfig = workloads.GenerateWGConfig(
			workloads.WgIP(*znet.ExternalIP).IP.String(),
			znet.ExternalSK.String(),
			d.Keys[znet.PublicNodeID].PublicKey().String(),
			fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], d.WGPort[znet.PublicNodeID]),
			znet.IPRange.String(),
		)
	}

	// accessible nodes deployments
	for _, nodeID := range accessibleNodes {
		peers := make([]zos.Peer, 0, len(znet.Nodes))
		for _, peerNodeID := range accessibleNodes {
			if peerNodeID == nodeID {
				continue
			}

			peerIPRange := znet.NodesIPRange[peerNodeID]
			allowedIPs := []gridtypes.IPNet{
				peerIPRange,
				workloads.WgIP(peerIPRange),
			}

			if peerNodeID == znet.PublicNodeID {
				allowedIPs = append(allowedIPs, nonAccessibleIPRanges...)
			}

			peers = append(peers, zos.Peer{
				Subnet:      znet.NodesIPRange[peerNodeID],
				WGPublicKey: d.Keys[peerNodeID].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[peerNodeID], d.WGPort[peerNodeID]),
				AllowedIPs:  allowedIPs,
			})
		}

		if nodeID == znet.PublicNodeID {
			// external node
			if znet.AddWGAccess {
				peers = append(peers, zos.Peer{
					Subnet:      *znet.ExternalIP,
					WGPublicKey: znet.ExternalSK.PublicKey().String(),
					AllowedIPs:  []gridtypes.IPNet{*znet.ExternalIP, workloads.WgIP(*znet.ExternalIP)},
				})
			}

			// hidden nodes
			for _, peerNodeID := range hiddenNodes {
				peerIPRange := znet.NodesIPRange[peerNodeID]
				peers = append(peers, zos.Peer{
					Subnet:      peerIPRange,
					WGPublicKey: d.Keys[peerNodeID].PublicKey().String(),
					AllowedIPs: []gridtypes.IPNet{
						peerIPRange,
						workloads.WgIP(peerIPRange),
					},
				})
			}
		}

		workload := znet.ZosWorkload(znet.NodesIPRange[nodeID], d.Keys[nodeID].String(), uint16(d.WGPort[nodeID]), peers)
		deployment := workloads.NewGridDeployment(d.tfPluginClient.twinID, []gridtypes.Workload{workload})

		// add metadata
		var err error
		deployment.Metadata, err = znet.GenerateMetadata()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate deployment %s metadata", znet.Name)
		}

		deployments[nodeID] = deployment
	}

	// hidden nodes deployments
	for _, nodeID := range hiddenNodes {
		peers := make([]zos.Peer, 0)
		if znet.PublicNodeID != 0 {
			peers = append(peers, zos.Peer{
				WGPublicKey: d.Keys[znet.PublicNodeID].PublicKey().String(),
				Subnet:      znet.NodesIPRange[nodeID],
				AllowedIPs: []gridtypes.IPNet{
					znet.IPRange,
					workloads.IPNet(100, 64, 0, 0, 16),
				},
				Endpoint: fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], d.WGPort[znet.PublicNodeID]),
			})
		}
		workload := znet.ZosWorkload(znet.NodesIPRange[nodeID], d.Keys[nodeID].String(), uint16(d.WGPort[nodeID]), peers)
		deployment := workloads.NewGridDeployment(d.tfPluginClient.twinID, []gridtypes.Workload{workload})

		// add metadata
		var err error
		deployment.Metadata, err = znet.GenerateMetadata()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate deployment %s metadata", znet.Name)
		}

		deployments[nodeID] = deployment
	}
	return deployments, nil
}

// Deploy deploys the network deployments using the deployer
func (d *NetworkDeployer) Deploy(ctx context.Context, znet *workloads.ZNet) error {
	err := d.Validate(ctx, znet)
	if err != nil {
		return err
	}

	newDeployments, err := d.GenerateVersionlessDeployments(ctx, znet)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	log.Println("new deployments")
	err = PrintDeployments(newDeployments)
	if err != nil {
		return errors.Wrap(err, "couldn't print deployments data")
	}

	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	for _, nodeID := range znet.Nodes {
		// solution providers
		newDeploymentsSolutionProvider[nodeID] = nil
	}

	currentDeployments, err := d.deployer.Deploy(ctx, d.tfPluginClient.StateLoader.currentNodeNetwork, newDeployments, newDeploymentsSolutionProvider)
	if err != nil {
		return errors.Wrapf(err, "couldn't deploy network %s", znet.Name)
	}

	// update state
	if znet.NodeDeploymentID == nil {
		znet.NodeDeploymentID = make(map[uint32]uint64)
	}

	for _, nodeID := range znet.Nodes {
		if currentDeployments[nodeID] != 0 {
			znet.NodeDeploymentID[nodeID] = currentDeployments[nodeID]
			d.tfPluginClient.StateLoader.networks.updateNetwork(znet.Name, znet.NodesIPRange)
			d.tfPluginClient.StateLoader.currentNodeNetwork[nodeID] = currentDeployments[nodeID]
		}
	}

	if err := d.readNodesConfig(ctx, znet); err != nil {
		return errors.Wrap(err, "couldn't read node's data")
	}

	return nil
}

// Cancel cancels all the deployments
func (d *NetworkDeployer) Cancel(ctx context.Context, znet *workloads.ZNet) error {
	err := d.Validate(ctx, znet)
	if err != nil {
		return err
	}

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeNetwork

	for nodeID, contractID := range oldDeployments {
		if workloads.Contains(znet.Nodes, nodeID) {
			err = d.deployer.Cancel(ctx, contractID)
			if err != nil {
				return errors.Wrapf(err, "couldn't cancel network %s, contract %d", znet.Name, contractID)
			}
			delete(znet.NodeDeploymentID, nodeID)
			delete(d.tfPluginClient.StateLoader.currentNodeNetwork, nodeID)
		}
	}

	// delete network from state if all contracts was deleted
	d.tfPluginClient.StateLoader.networks.deleteNetwork(znet.Name)

	if err := d.readNodesConfig(ctx, znet); err != nil {
		return errors.Wrap(err, "couldn't read node's data")
	}

	return nil
}

// invalidateBrokenAttributes removes outdated attrs and deleted contracts
func (d *NetworkDeployer) invalidateBrokenAttributes(znet *workloads.ZNet) error {
	for node, contractID := range znet.NodeDeploymentID {
		contract, err := d.tfPluginClient.SubstrateConn.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, substrate.ErrNotFound) {
			delete(znet.NodeDeploymentID, node)
			delete(znet.NodesIPRange, node)
			delete(d.Keys, node)
			delete(d.WGPort, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		}
	}
	if znet.ExternalIP != nil && !znet.IPRange.Contains(znet.ExternalIP.IP) {
		znet.ExternalIP = nil
	}
	for node, ip := range znet.NodesIPRange {
		if !znet.IPRange.Contains(ip.IP) {
			delete(znet.NodesIPRange, node)
		}
	}
	if znet.PublicNodeID != 0 {
		// TODO: add a check that the node is still public
		cl, err := d.tfPluginClient.NcPool.GetNodeClient(d.tfPluginClient.SubstrateConn, znet.PublicNodeID)
		if err != nil {
			// whatever the error, delete it and it will get reassigned later
			znet.PublicNodeID = 0
		}
		if err := cl.IsNodeUp(context.Background()); err != nil {
			znet.PublicNodeID = 0
		}
	}

	if !znet.AddWGAccess {
		znet.ExternalIP = nil
	}
	return nil
}

func (d *NetworkDeployer) assignNodesIPs(nodes []uint32, znet *workloads.ZNet) error {
	ips := make(map[uint32]gridtypes.IPNet)
	l := len(znet.IPRange.IP)
	usedIPs := make([]byte, 0) // the third octet
	for node, ip := range znet.NodesIPRange {
		if workloads.Contains(nodes, node) {
			usedIPs = append(usedIPs, ip.IP[l-2])
			ips[node] = ip
		}
	}
	var cur byte = 2
	if znet.AddWGAccess {
		if znet.ExternalIP != nil {
			usedIPs = append(usedIPs, znet.ExternalIP.IP[l-2])
		} else {
			err := workloads.NextFreeIP(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ip := workloads.IPNet(znet.IPRange.IP[l-4], znet.IPRange.IP[l-3], cur, znet.IPRange.IP[l-1], 24)
			znet.ExternalIP = &ip
		}
	}
	for _, nodeID := range nodes {
		if _, ok := ips[nodeID]; !ok {
			err := workloads.NextFreeIP(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ips[nodeID] = workloads.IPNet(znet.IPRange.IP[l-4], znet.IPRange.IP[l-3], cur, znet.IPRange.IP[l-2], 24)
		}
	}
	znet.NodesIPRange = ips
	return nil
}

func (d *NetworkDeployer) assignNodesWGPort(ctx context.Context, sub subi.SubstrateExt, nodes []uint32) error {
	for _, nodeID := range nodes {
		if _, ok := d.WGPort[nodeID]; !ok {
			cl, err := d.tfPluginClient.NcPool.GetNodeClient(sub, nodeID)
			if err != nil {
				return errors.Wrap(err, "could not get node client")
			}
			port, err := workloads.GetNodeFreeWGPort(ctx, cl, nodeID)
			if err != nil {
				return errors.Wrap(err, "failed to get node free wg ports")
			}
			d.WGPort[nodeID] = port
		}
	}

	return nil
}

func (d *NetworkDeployer) assignNodesWGKey(nodes []uint32) error {
	for _, nodeID := range nodes {
		if _, ok := d.Keys[nodeID]; !ok {

			key, err := wgtypes.GenerateKey()
			if err != nil {
				return errors.Wrap(err, "failed to generate wg private key")
			}
			d.Keys[nodeID] = key
		}
	}

	return nil
}

func (d *NetworkDeployer) readNodesConfig(ctx context.Context, znet *workloads.ZNet) error {
	keys := make(map[uint32]wgtypes.Key)
	WGPort := make(map[uint32]int)
	nodesIPRange := make(map[uint32]gridtypes.IPNet)
	log.Printf("reading node config")
	nodeDeployments, err := d.deployer.GetDeployments(ctx, znet.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "failed to get deployment objects")
	}
	err = PrintDeployments(nodeDeployments)
	if err != nil {
		return errors.Wrap(err, "failed to print deployments")
	}

	WGAccess := false
	for node, dl := range nodeDeployments {
		for _, wl := range dl.Workloads {
			if wl.Type != zos.NetworkType {
				continue
			}
			data, err := wl.WorkloadData()
			if err != nil {
				return errors.Wrap(err, "couldn't parse workload data")
			}

			d := data.(*zos.Network)
			WGPort[node] = int(d.WGListenPort)
			keys[node], err = wgtypes.ParseKey(d.WGPrivateKey)
			if err != nil {
				return errors.Wrap(err, "couldn't parse wg private key from workload object")
			}
			nodesIPRange[node] = d.Subnet
			// this will fail when hidden node is supported
			for _, peer := range d.Peers {
				if peer.Endpoint == "" {
					WGAccess = true
				}
			}
		}
	}
	d.Keys = keys
	d.WGPort = WGPort
	znet.NodesIPRange = nodesIPRange
	znet.AddWGAccess = WGAccess
	if !WGAccess {
		znet.AccessWGConfig = ""
	}
	return nil
}
