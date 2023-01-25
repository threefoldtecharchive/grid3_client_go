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

	TFPluginClient *TFPluginClient
	deployer       Deployer
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(tfPluginClient *TFPluginClient) NetworkDeployer {
	netDeployer := NetworkDeployer{
		Keys:           map[uint32]wgtypes.Key{},
		WGPort:         map[uint32]int{},
		TFPluginClient: tfPluginClient,
		deployer:       Deployer{},
	}

	netDeployer.TFPluginClient = tfPluginClient
	netDeployer.deployer = NewDeployer(*tfPluginClient, true)

	return netDeployer
}

// GenerateComputedZNet generates the computed fields in the given znet
func (k *NetworkDeployer) GenerateComputedZNet(ctx context.Context, tfPluginClient *TFPluginClient, znet *workloads.ZNet, solutionType string) (*workloads.ZNet, error) {
	if solutionType == "" {
		solutionType = "NETWORK"
	}
	znet.SolutionType = solutionType

	peerSubnets := map[string]*zos.Peer{}
	nodeSubnets := map[string]bool{}

	// retrieve last network state
	oldContracts := k.deployer.stateLoader.currentNodeDeployment

	for _, nodeID := range znet.Nodes {
		if _, ok := oldContracts[nodeID]; !ok {
			// if node is new, it has no previous state and shouldn't be processed
			continue
		}
		delete(oldContracts, nodeID)

		dl, err := k.deployer.stateLoader.getDeployment(nodeID)
		if err != nil {
			return &workloads.ZNet{}, errors.Wrapf(err, "couldn't get deployment with node ID %d", nodeID)
		}

		for _, wl := range dl.Workloads {
			if wl.Name.String() == znet.Name && wl.Result.State == gridtypes.StateOk {
				data, err := workloads.GetZNetWorkloadData(wl)
				if err != nil {
					return &workloads.ZNet{}, errors.Wrapf(err, "couldn't get workload \"%s\" data", wl.Name.String())
				}

				privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
				if err != nil {
					return &workloads.ZNet{}, errors.Wrap(err, "couldn't parse wire guard private key")
				}

				for idx, peer := range data.Peers {
					peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
					if znet.AddWGAccess && peer.Endpoint == "" {
						// this is the access node
						znet.PublicNodeID = nodeID
					}
				}
				nodeSubnets[data.Subnet.String()] = true

				znet.NodeDeploymentID[nodeID] = dl.ContractID
				k.Keys[nodeID] = privateKey
				k.WGPort[nodeID] = int(data.WGListenPort)
				znet.NodesIPRange[nodeID] = data.Subnet
			}
		}

	}
	// TODO: if oldDeployments is not empty and has any of this networks' workloads, they should be canceled
	// if the workload represents an access node, and the user requires wg access, the workload should not be cancelled
	//toCancel := map[uint32]map[string]bool{}

	for nodeID, contractID := range oldContracts {
		dl, err := k.deployer.stateLoader.getDeployment(nodeID)
		if err != nil {
			return &workloads.ZNet{}, errors.Wrapf(err, "couldn't get deployment %d", contractID)
		}
		wlID, err := dl.Get(gridtypes.Name(znet.Name))

		if err == nil {
			wl := *(wlID.Workload)
			data, err := workloads.GetZNetWorkloadData(wl)
			if err != nil {
				return &workloads.ZNet{}, errors.New("could not create network workload")
			}
			privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
			if err != nil {
				return &workloads.ZNet{}, errors.Wrap(err, "couldn't parse private key")
			}

			for idx, peer := range data.Peers {
				peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
				if znet.AddWGAccess && peer.Endpoint == "" {
					// this is the access node
					znet.PublicNodeID = nodeID
					znet.NodeDeploymentID[nodeID] = dl.ContractID
					k.Keys[nodeID] = privateKey
					k.WGPort[nodeID] = int(data.WGListenPort)
					znet.NodesIPRange[nodeID] = data.Subnet
					break
				}
			}
			nodeSubnets[data.Subnet.String()] = true

			/*
				if !znet.AddWGAccess || znet.PublicNodeID != nodeID {
					// this is a node to be cancelled
					toCancel[nodeID] = make(map[string]bool)
					toCancel[nodeID][znet.Name] = true
				}
			*/

		}
	}

	for subnet := range peerSubnets {
		if _, ok := nodeSubnets[subnet]; !ok {
			// this was the user access ip
			externalIP, err := gridtypes.ParseIPNet(subnet)
			if err != nil {
				return &workloads.ZNet{}, errors.Wrapf(err, "couldn't parse user address")
			}
			znet.ExternalIP = &externalIP
			break
		}
	}

	if znet.ExternalIP == nil {
		// user does not have userAccess configs and a new private key should be generated
		secretKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return &workloads.ZNet{}, errors.Wrapf(err, "couldn't generate new private key")
		}
		znet.ExternalSK = secretKey
	}

	return znet, nil
}

// Validate validates a network deployer
func (k *NetworkDeployer) Validate(ctx context.Context, znet *workloads.ZNet) error {
	sub := k.TFPluginClient.SubstrateConn

	if err := validateAccountBalanceForExtrinsics(sub, k.TFPluginClient.Identity); err != nil {
		return err
	}

	if err := znet.Validate(); err != nil {
		return err
	}

	err := client.AreNodesUp(ctx, sub, znet.Nodes, k.deployer.ncPool)
	if err != nil {
		return err
	}

	return k.invalidateBrokenAttributes(sub, znet)
}

// GenerateVersionlessDeploymentsAndWorkloads generates deployments for network deployer without versions
func (k *NetworkDeployer) GenerateVersionlessDeploymentsAndWorkloads(ctx context.Context, znet *workloads.ZNet) (map[uint32]gridtypes.Deployment, map[uint32][]gridtypes.Workload, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	netWorkloads := make(map[uint32][]gridtypes.Workload)

	err := k.Validate(ctx, znet)
	if err != nil {
		return nil, nil, err
	}

	log.Printf("nodes: %v\n", znet.Nodes)
	sub := k.TFPluginClient.SubstrateConn

	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	accessibleNodes := make([]uint32, 0)
	var ipv4Node uint32

	for _, nodeID := range znet.Nodes {
		nodeClient, err := k.deployer.ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "couldn't get node %d client", nodeID)
		}

		endpoint, err := workloads.GetNodeEndpoint(ctx, nodeClient)
		if errors.Is(err, workloads.ErrNoAccessibleInterfaceFound) {
			hiddenNodes = append(hiddenNodes, nodeID)
		} else if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to get node %d endpoint", nodeID)
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
			publicNode, err := workloads.GetPublicNode(ctx, k.TFPluginClient.GridProxyClient, []uint32{})
			if err != nil {
				return nil, nil, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			znet.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}

		if endpoints[znet.PublicNodeID] == "" { // old or new outsider
			cl, err := k.deployer.ncPool.GetNodeClient(sub, znet.PublicNodeID)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "couldn't get node %d client", znet.PublicNodeID)
			}
			endpoint, err := workloads.GetNodeEndpoint(ctx, cl)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to get node %d endpoint", znet.PublicNodeID)
			}
			endpoints[znet.PublicNodeID] = endpoint.String()
		}
	}

	allNodes := append(hiddenNodes, accessibleNodes...)
	if err := k.assignNodesIPs(allNodes, znet); err != nil {
		return nil, nil, errors.Wrap(err, "couldn't assign node ips")
	}
	if err := k.assignNodesWGKey(allNodes); err != nil {
		return nil, nil, errors.Wrap(err, "couldn't assign node wg keys")
	}
	if err := k.assignNodesWGPort(ctx, sub, allNodes); err != nil {
		return nil, nil, errors.Wrap(err, "couldn't assign node wg ports")
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
			k.Keys[znet.PublicNodeID].PublicKey().String(),
			fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], k.WGPort[znet.PublicNodeID]),
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
				WGPublicKey: k.Keys[peerNodeID].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[peerNodeID], k.WGPort[peerNodeID]),
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
					WGPublicKey: k.Keys[peerNodeID].PublicKey().String(),
					AllowedIPs: []gridtypes.IPNet{
						peerIPRange,
						workloads.WgIP(peerIPRange),
					},
				})
			}
		}

		workload := znet.GenerateWorkload(znet.NodesIPRange[nodeID], k.Keys[nodeID].String(), uint16(k.WGPort[nodeID]), peers)
		netWorkloads[nodeID] = append(netWorkloads[nodeID], workload)

		deployment := workloads.NewGridDeployment(k.TFPluginClient.TwinID, []gridtypes.Workload{workload})
		deployments[nodeID] = deployment
	}

	// hidden nodes deployments
	for _, nodeID := range hiddenNodes {
		peers := make([]zos.Peer, 0)
		if znet.PublicNodeID != 0 {
			peers = append(peers, zos.Peer{
				WGPublicKey: k.Keys[znet.PublicNodeID].PublicKey().String(),
				Subnet:      znet.NodesIPRange[nodeID],
				AllowedIPs: []gridtypes.IPNet{
					znet.IPRange,
					workloads.IPNet(100, 64, 0, 0, 16),
				},
				Endpoint: fmt.Sprintf("%s:%d", endpoints[znet.PublicNodeID], k.WGPort[znet.PublicNodeID]),
			})
		}

		workload := znet.GenerateWorkload(znet.NodesIPRange[nodeID], k.Keys[nodeID].String(), uint16(k.WGPort[nodeID]), peers)
		netWorkloads[nodeID] = append(netWorkloads[nodeID], workload)

		deployment := workloads.NewGridDeployment(k.TFPluginClient.TwinID, []gridtypes.Workload{workload})
		deployments[nodeID] = deployment
	}
	return deployments, netWorkloads, nil
}

// Deploy deploys the network deployments using the deployer
func (k *NetworkDeployer) Deploy(ctx context.Context, znet *workloads.ZNet) error {
	sub := k.TFPluginClient.SubstrateConn

	newDeployments, _, err := k.GenerateVersionlessDeploymentsAndWorkloads(ctx, znet)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	log.Println("new deployments")
	err = PrintDeployments(newDeployments)
	if err != nil {
		return errors.Wrap(err, "couldn't print deployments data")
	}

	deploymentData := DeploymentData{
		Name:        znet.Name,
		Type:        "network",
		ProjectName: znet.SolutionType,
	}
	// deployment data
	newDeploymentsData := map[uint32]DeploymentData{znet.PublicNodeID: deploymentData}

	// solution providers
	newDeploymentsSolutionProvider := map[uint32]*uint64{znet.PublicNodeID: nil}

	currentDeployments, err := k.deployer.Deploy(ctx, sub, znet.NodeDeploymentID, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

	znet.NodeDeploymentID = currentDeployments
	k.deployer.stateLoader.networks.updateNetwork(znet.Name, znet.NodesIPRange)
	if err := k.readNodesConfig(ctx, sub, znet); err != nil {
		return errors.Wrap(err, "couldn't read node's data")
	}

	return err
}

// Cancel cancels all the deployments
func (k *NetworkDeployer) Cancel(ctx context.Context, sub subi.SubstrateExt, znet *workloads.ZNet) error {
	newDeployments := map[uint32]gridtypes.Deployment{znet.PublicNodeID: {}}

	deploymentData := DeploymentData{
		Name:        znet.Name,
		Type:        "network",
		ProjectName: znet.SolutionType,
	}
	// deployment data
	newDeploymentsData := map[uint32]DeploymentData{znet.PublicNodeID: deploymentData}

	// solution providers
	newDeploymentsSolutionProvider := map[uint32]*uint64{znet.PublicNodeID: nil}

	currentDeployments, err := k.deployer.Deploy(ctx, sub, znet.NodeDeploymentID, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

	znet.NodeDeploymentID = currentDeployments
	k.deployer.stateLoader.networks.updateNetwork(znet.Name, znet.NodesIPRange)
	if err := k.readNodesConfig(ctx, sub, znet); err != nil {
		return errors.Wrap(err, "couldn't read node's data")
	}
	return err
}

// invalidateBrokenAttributes removes outdated attrs and deleted contracts
func (k *NetworkDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt, znet *workloads.ZNet) error {
	for node, contractID := range znet.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, substrate.ErrNotFound) {
			delete(znet.NodeDeploymentID, node)
			delete(znet.NodesIPRange, node)
			delete(k.Keys, node)
			delete(k.WGPort, node)
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
		cl, err := k.deployer.ncPool.GetNodeClient(sub, znet.PublicNodeID)
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

func (k *NetworkDeployer) assignNodesIPs(nodes []uint32, znet *workloads.ZNet) error {
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
			err := workloads.NextFreeOctet(usedIPs, &cur)
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
			err := workloads.NextFreeOctet(usedIPs, &cur)
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

func (k *NetworkDeployer) assignNodesWGPort(ctx context.Context, sub subi.SubstrateExt, nodes []uint32) error {
	for _, nodeID := range nodes {
		if _, ok := k.WGPort[nodeID]; !ok {
			cl, err := k.deployer.ncPool.GetNodeClient(sub, nodeID)
			if err != nil {
				return errors.Wrap(err, "could not get node client")
			}
			port, err := workloads.GetNodeFreeWGPort(ctx, cl, nodeID)
			if err != nil {
				return errors.Wrap(err, "failed to get node free wg ports")
			}
			k.WGPort[nodeID] = port
		}
	}

	return nil
}

func (k *NetworkDeployer) assignNodesWGKey(nodes []uint32) error {
	for _, nodeID := range nodes {
		if _, ok := k.Keys[nodeID]; !ok {

			key, err := wgtypes.GenerateKey()
			if err != nil {
				return errors.Wrap(err, "failed to generate wg private key")
			}
			k.Keys[nodeID] = key
		}
	}

	return nil
}

func (k *NetworkDeployer) readNodesConfig(ctx context.Context, sub subi.SubstrateExt, znet *workloads.ZNet) error {
	keys := make(map[uint32]wgtypes.Key)
	WGPort := make(map[uint32]int)
	nodesIPRange := make(map[uint32]gridtypes.IPNet)
	log.Printf("reading node config")
	nodeDeployments, err := k.deployer.GetDeployments(ctx, sub, znet.NodeDeploymentID)
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
	k.Keys = keys
	k.WGPort = WGPort
	znet.NodesIPRange = nodesIPRange
	znet.AddWGAccess = WGAccess
	if !WGAccess {
		znet.AccessWGConfig = ""
	}
	return nil
}
