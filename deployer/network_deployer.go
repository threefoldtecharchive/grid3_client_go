// Package deployer for grid deployer
package deployer

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// NetworkDeployer struct
type NetworkDeployer struct {
	tfPluginClient *TFPluginClient
	deployer       MockDeployer
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(tfPluginClient *TFPluginClient) NetworkDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	return NetworkDeployer{
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}
}

// Validate validates a network deployer
func (d *NetworkDeployer) Validate(ctx context.Context, znet *workloads.ZNet) error {
	sub := d.tfPluginClient.SubstrateConn

	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.Identity); err != nil {
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
			return nil, errors.Wrapf(err, "could not get node %d client", nodeID)
		}

		endpoint, err := nodeClient.GetNodeEndpoint(ctx)
		if errors.Is(err, client.ErrNoAccessibleInterfaceFound) {
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
			publicNode, err := GetPublicNode(ctx, d.tfPluginClient.GridProxyClient, []uint32{})
			if err != nil {
				return nil, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			znet.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}

		if endpoints[znet.PublicNodeID] == "" { // old or new outsider
			cl, err := d.tfPluginClient.NcPool.GetNodeClient(sub, znet.PublicNodeID)
			if err != nil {
				return nil, errors.Wrapf(err, "could not get node %d client", znet.PublicNodeID)
			}
			endpoint, err := cl.GetNodeEndpoint(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get node %d endpoint", znet.PublicNodeID)
			}
			endpoints[znet.PublicNodeID] = endpoint.String()
		}
	}

	allNodes := append(hiddenNodes, accessibleNodes...)
	if err := znet.AssignNodesIPs(allNodes); err != nil {
		return nil, errors.Wrap(err, "could not assign node ips")
	}
	if err := znet.AssignNodesWGKey(allNodes); err != nil {
		return nil, errors.Wrap(err, "could not assign node wg keys")
	}
	if err := znet.AssignNodesWGPort(ctx, sub, d.tfPluginClient.NcPool, allNodes); err != nil {
		return nil, errors.Wrap(err, "could not assign node wg ports")
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
			znet.Keys[znet.PublicNodeID].PublicKey().String(),
			fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], znet.WGPort[znet.PublicNodeID]),
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
				WGPublicKey: znet.Keys[peerNodeID].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[peerNodeID], znet.WGPort[peerNodeID]),
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
					WGPublicKey: znet.Keys[peerNodeID].PublicKey().String(),
					AllowedIPs: []gridtypes.IPNet{
						peerIPRange,
						workloads.WgIP(peerIPRange),
					},
				})
			}
		}

		workload := znet.ZosWorkload(znet.NodesIPRange[nodeID], znet.Keys[nodeID].String(), uint16(znet.WGPort[nodeID]), peers)
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
				WGPublicKey: znet.Keys[znet.PublicNodeID].PublicKey().String(),
				Subnet:      znet.NodesIPRange[nodeID],
				AllowedIPs: []gridtypes.IPNet{
					znet.IPRange,
					workloads.IPNet(100, 64, 0, 0, 16),
				},
				Endpoint: fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], znet.WGPort[znet.PublicNodeID]),
			})
		}
		workload := znet.ZosWorkload(znet.NodesIPRange[nodeID], znet.Keys[nodeID].String(), uint16(znet.WGPort[nodeID]), peers)
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
		return errors.Wrap(err, "could not generate deployments data")
	}

	log.Println("new deployments")
	err = PrintDeployments(newDeployments)
	if err != nil {
		return errors.Wrap(err, "could not print deployments data")
	}

	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	for _, nodeID := range znet.Nodes {
		// solution providers
		newDeploymentsSolutionProvider[nodeID] = nil
	}

	znet.NodeDeploymentID, err = d.deployer.Deploy(ctx, znet.NodeDeploymentID, newDeployments, newDeploymentsSolutionProvider)

	// update deployment and plugin state
	// error is not returned immediately before updating state because of untracked failed deployments
	for _, nodeID := range znet.Nodes {
		if contractID, ok := znet.NodeDeploymentID[nodeID]; ok && contractID != 0 {
			d.tfPluginClient.State.networks.updateNetwork(znet.Name, znet.NodesIPRange)
			if !workloads.Contains(d.tfPluginClient.State.currentNodeDeployments[nodeID], znet.NodeDeploymentID[nodeID]) {
				d.tfPluginClient.State.currentNodeNetworks[nodeID] = append(d.tfPluginClient.State.currentNodeNetworks[nodeID], znet.NodeDeploymentID[nodeID])
			}
		}
	}

	if err != nil {
		return errors.Wrapf(err, "could not deploy network %s", znet.Name)
	}

	if err := d.readNodesConfig(ctx, znet); err != nil {
		return errors.Wrap(err, "could not read node's data")
	}

	return nil
}

// Cancel cancels all the deployments
func (d *NetworkDeployer) Cancel(ctx context.Context, znet *workloads.ZNet) error {
	err := d.Validate(ctx, znet)
	if err != nil {
		return err
	}

	for nodeID, contractID := range znet.NodeDeploymentID {
		if workloads.Contains(znet.Nodes, nodeID) {
			err = d.deployer.Cancel(ctx, contractID)
			if err != nil {
				return errors.Wrapf(err, "could not cancel network %s, contract %d", znet.Name, contractID)
			}
			delete(znet.NodeDeploymentID, nodeID)
			d.tfPluginClient.State.currentNodeDeployments[nodeID] = workloads.Delete(d.tfPluginClient.State.currentNodeDeployments[nodeID], contractID)
		}
	}

	// delete network from state if all contracts was deleted
	d.tfPluginClient.State.networks.deleteNetwork(znet.Name)

	if err := d.readNodesConfig(ctx, znet); err != nil {
		return errors.Wrap(err, "could not read node's data")
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
			delete(znet.Keys, node)
			delete(znet.WGPort, node)
		} else if err != nil {
			return errors.Wrapf(err, "could not get node %d contract %d", node, contractID)
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
				return errors.Wrap(err, "could not parse workload data")
			}

			d := data.(*zos.Network)
			WGPort[node] = int(d.WGListenPort)
			keys[node], err = wgtypes.ParseKey(d.WGPrivateKey)
			if err != nil {
				return errors.Wrap(err, "could not parse wg private key from workload object")
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
	znet.Keys = keys
	znet.WGPort = WGPort
	znet.NodesIPRange = nodesIPRange
	znet.AddWGAccess = WGAccess
	if !WGAccess {
		znet.AccessWGConfig = ""
	}
	return nil
}
