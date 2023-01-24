// Package manager for grid manager
package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
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
	ZNet         workloads.ZNet
	solutionType string

	AccessWGConfig   string
	ExternalIP       *gridtypes.IPNet
	ExternalSK       wgtypes.Key
	ExternalPK       wgtypes.Key
	PublicNodeID     uint32
	NodeDeploymentID map[uint32]uint64
	NodesIPRange     map[uint32]gridtypes.IPNet
	WGPort           map[uint32]int
	Keys             map[uint32]wgtypes.Key

	TFPluginClient *TFPluginClient
	ncPool         *client.NodeClientPool
	deployer       deployer.Deployer
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(ctx context.Context, znet workloads.ZNet, solutionType string, tfPluginClient *TFPluginClient) (NetworkDeployer, error) {
	netDeployer := NetworkDeployer{
		ZNet:             znet,
		solutionType:     solutionType,
		AccessWGConfig:   "",
		ExternalIP:       &gridtypes.IPNet{},
		ExternalSK:       wgtypes.Key{},
		PublicNodeID:     uint32(0),
		NodesIPRange:     map[uint32]gridtypes.IPNet{},
		NodeDeploymentID: map[uint32]uint64{},
		Keys:             map[uint32]wgtypes.Key{},
		WGPort:           map[uint32]int{},
		TFPluginClient:   tfPluginClient,
		ncPool:           &client.NodeClientPool{},
		deployer:         deployer.Deployer{},
	}

	peerSubnets := map[string]*zos.Peer{}
	nodeSubnets := map[string]bool{}

	// retrieve last network state
	oldContracts := tfPluginClient.manager.GetContractIDs()

	for _, nodeID := range znet.Nodes {
		if _, ok := oldContracts[nodeID]; !ok {
			// if node is new, it has no previous state and shouldn't be processed
			continue
		}
		delete(oldContracts, nodeID)

		dl, err := tfPluginClient.manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment with node ID %d", nodeID)
		}

		for _, wl := range dl.Workloads {
			if wl.Name.String() == znet.Name && wl.Result.State == gridtypes.StateOk {
				data, err := workloads.GetZNetWorkloadData(wl)
				if err != nil {
					return NetworkDeployer{}, errors.Wrapf(err, "couldn't get workload \"%s\" data", wl.Name.String())
				}

				privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
				if err != nil {
					return NetworkDeployer{}, errors.Wrap(err, "couldn't parse wire guard private key")
				}

				for idx, peer := range data.Peers {
					peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
					if znet.AddWGAccess && peer.Endpoint == "" {
						// this is the access node
						netDeployer.PublicNodeID = nodeID
					}
				}
				nodeSubnets[data.Subnet.String()] = true

				netDeployer.NodeDeploymentID[nodeID] = dl.ContractID
				netDeployer.Keys[nodeID] = privateKey
				netDeployer.WGPort[nodeID] = int(data.WGListenPort)
				netDeployer.NodesIPRange[nodeID] = data.Subnet
			}
		}

	}
	// TODO: if oldDeployments is not empty and has any of this networks' workloads, they should be canceled
	// if the workload represents an access node, and the user requires wg access, the workload should not be cancelled
	toCancel := map[uint32]map[string]bool{}
	for nodeID, contractID := range oldContracts {
		dl, err := tfPluginClient.manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment %d", contractID)
		}
		wlID, err := dl.Get(gridtypes.Name(znet.Name))

		if err == nil {
			wl := *(wlID.Workload)
			data, err := workloads.GetZNetWorkloadData(wl)
			if err != nil {
				return NetworkDeployer{}, errors.New("could not create network workload")
			}
			privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrap(err, "couldn't parse private key")
			}

			for idx, peer := range data.Peers {
				peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
				if znet.AddWGAccess && peer.Endpoint == "" {
					// this is the access node
					netDeployer.PublicNodeID = nodeID
					netDeployer.NodeDeploymentID[nodeID] = dl.ContractID
					netDeployer.Keys[nodeID] = privateKey
					netDeployer.WGPort[nodeID] = int(data.WGListenPort)
					netDeployer.NodesIPRange[nodeID] = data.Subnet
					break
				}
			}
			nodeSubnets[data.Subnet.String()] = true

			if !znet.AddWGAccess || netDeployer.PublicNodeID != nodeID {
				// this is a node to be cancelled
				toCancel[nodeID] = make(map[string]bool)
				toCancel[nodeID][znet.Name] = true
			}

		}
	}
	for subnet, peer := range peerSubnets {
		if _, ok := nodeSubnets[subnet]; !ok {
			// this was the user access ip
			externalIP, err := gridtypes.ParseIPNet(subnet)
			if err != nil {
				return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse user address")
			}
			netDeployer.ExternalIP = &externalIP
			pk, err := wgtypes.ParseKey(peer.WGPublicKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse peer wg public key")
			}
			netDeployer.ExternalPK = pk
			break
		}
	}

	if netDeployer.ExternalIP == nil {
		// user does not have userAccess configs and a new private key should be generated
		secretKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't generate new private key")
		}
		netDeployer.ExternalSK = secretKey
		netDeployer.ExternalPK = secretKey.PublicKey()
	}

	err := tfPluginClient.manager.CancelWorkloads(toCancel)
	if err != nil {
		return NetworkDeployer{}, errors.Wrapf(err, "couldn't cancel workloads")
	}

	netDeployer.ncPool = client.NewNodeClientPool(tfPluginClient.rmb)

	if solutionType == "" {
		solutionType = "NETWORK"
	}
	deploymentData := DeploymentData{
		Name:        znet.Name,
		Type:        "network",
		ProjectName: solutionType,
	}
	deploymentDataStr, err := json.Marshal(deploymentData)
	if err != nil {
		log.Printf("error parsing deployment data: %s", err.Error())
	}

	netDeployer.deployer = deployer.NewDeployer(tfPluginClient.identity, tfPluginClient.twinID, tfPluginClient.gridProxyClient, netDeployer.ncPool, true, nil, string(deploymentDataStr))

	return netDeployer, nil
}

// Validate validates a network deployer
func (k *NetworkDeployer) Validate(ctx context.Context) error {
	sub := k.TFPluginClient.substrateConn

	if err := validateAccountBalanceForExtrinsics(sub, k.TFPluginClient.identity); err != nil {
		return err
	}

	mask := k.ZNet.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in ip range %s should be 16", k.ZNet.IPRange.String())
	}

	return client.AreNodesUp(ctx, sub, k.ZNet.Nodes, k.ncPool)
}

// GenerateVersionlessDeploymentsAndWorkloads generates deployments for network deployer without versions
func (k *NetworkDeployer) GenerateVersionlessDeploymentsAndWorkloads(ctx context.Context) (map[uint32]gridtypes.Deployment, map[uint32][]gridtypes.Workload, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	netWorkloads := make(map[uint32][]gridtypes.Workload)

	err := k.Validate(ctx)
	if err != nil {
		return nil, nil, err
	}

	log.Printf("nodes: %v\n", k.ZNet.Nodes)
	sub := k.TFPluginClient.substrateConn

	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	accessibleNodes := make([]uint32, 0)
	var ipv4Node uint32

	for _, nodeID := range k.ZNet.Nodes {
		nodeClient, err := k.ncPool.GetNodeClient(sub, nodeID)
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

	needsIPv4Access := k.ZNet.AddWGAccess || (len(hiddenNodes) != 0 && len(hiddenNodes)+len(accessibleNodes) > 1)
	if needsIPv4Access {
		if k.PublicNodeID != 0 { // it's set
			// if public node id is already set, it should be added to accessible nodes
			if !workloads.Contains(accessibleNodes, k.PublicNodeID) {
				accessibleNodes = append(accessibleNodes, k.PublicNodeID)
			}
		} else if ipv4Node != 0 { // there's one in the network original nodes
			k.PublicNodeID = ipv4Node
		} else {
			publicNode, err := workloads.GetPublicNode(ctx, k.TFPluginClient.gridProxyClient, []uint32{})
			if err != nil {
				return nil, nil, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			k.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}

		if endpoints[k.PublicNodeID] == "" { // old or new outsider
			cl, err := k.ncPool.GetNodeClient(sub, k.PublicNodeID)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "couldn't get node %d client", k.PublicNodeID)
			}
			endpoint, err := workloads.GetNodeEndpoint(ctx, cl)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to get node %d endpoint", k.PublicNodeID)
			}
			endpoints[k.PublicNodeID] = endpoint.String()
		}
	}

	allNodes := append(hiddenNodes, accessibleNodes...)
	if err := k.assignNodesIPs(allNodes); err != nil {
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
		r := k.NodesIPRange[nodeID]
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(r))
	}
	if k.ZNet.AddWGAccess {
		r := k.ExternalIP
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, *r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(*r))
	}

	log.Printf("hidden nodes: %v\n", hiddenNodes)
	log.Printf("public node: %v\n", k.PublicNodeID)
	log.Printf("accessible nodes: %v\n", accessibleNodes)
	log.Printf("non accessible ip ranges: %v\n", nonAccessibleIPRanges)

	if k.ZNet.AddWGAccess {
		k.AccessWGConfig = workloads.GenerateWGConfig(
			workloads.WgIP(*k.ExternalIP).IP.String(),
			k.ExternalSK.String(),
			k.Keys[k.PublicNodeID].PublicKey().String(),
			fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID]),
			k.ZNet.IPRange.String(),
		)
	}

	// accessible nodes deployments
	for _, nodeID := range accessibleNodes {
		peers := make([]zos.Peer, 0, len(k.ZNet.Nodes))
		for _, peerNodeID := range accessibleNodes {
			if peerNodeID == nodeID {
				continue
			}

			peerIPRange := k.NodesIPRange[peerNodeID]
			allowedIPs := []gridtypes.IPNet{
				peerIPRange,
				workloads.WgIP(peerIPRange),
			}

			if peerNodeID == k.PublicNodeID {
				allowedIPs = append(allowedIPs, nonAccessibleIPRanges...)
			}

			peers = append(peers, zos.Peer{
				Subnet:      k.NodesIPRange[peerNodeID],
				WGPublicKey: k.Keys[peerNodeID].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[peerNodeID], k.WGPort[peerNodeID]),
				AllowedIPs:  allowedIPs,
			})
		}

		if nodeID == k.PublicNodeID {
			// external node
			if k.ZNet.AddWGAccess {
				peers = append(peers, zos.Peer{
					Subnet:      *k.ExternalIP,
					WGPublicKey: k.ExternalSK.PublicKey().String(),
					AllowedIPs:  []gridtypes.IPNet{*k.ExternalIP, workloads.WgIP(*k.ExternalIP)},
				})
			}

			// hidden nodes
			for _, peerNodeID := range hiddenNodes {
				peerIPRange := k.NodesIPRange[peerNodeID]
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

		workload := k.ZNet.GenerateWorkload(k.NodesIPRange[nodeID], k.Keys[nodeID].String(), uint16(k.WGPort[nodeID]), peers)
		netWorkloads[nodeID] = append(netWorkloads[nodeID], workload)

		deployment := workloads.NewDeployment(k.TFPluginClient.twinID, []gridtypes.Workload{workload})
		deployments[nodeID] = deployment
	}

	// hidden nodes deployments
	for _, nodeID := range hiddenNodes {
		peers := make([]zos.Peer, 0)
		if k.PublicNodeID != 0 {
			peers = append(peers, zos.Peer{
				WGPublicKey: k.Keys[k.PublicNodeID].PublicKey().String(),
				Subnet:      k.NodesIPRange[nodeID],
				AllowedIPs: []gridtypes.IPNet{
					k.ZNet.IPRange,
					workloads.IPNet(100, 64, 0, 0, 16),
				},
				Endpoint: fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID]),
			})
		}

		workload := k.ZNet.GenerateWorkload(k.NodesIPRange[nodeID], k.Keys[nodeID].String(), uint16(k.WGPort[nodeID]), peers)
		netWorkloads[nodeID] = append(netWorkloads[nodeID], workload)

		deployment := workloads.NewDeployment(k.TFPluginClient.twinID, []gridtypes.Workload{workload})
		deployments[nodeID] = deployment
	}
	return deployments, netWorkloads, nil
}

// Deploy deploys the network deployments using the deployer
func (k *NetworkDeployer) Deploy(ctx context.Context) error {
	sub := k.TFPluginClient.substrateConn

	newDeployments, _, err := k.GenerateVersionlessDeploymentsAndWorkloads(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	log.Println("new deployments")
	err = deployer.PrintDeployments(newDeployments)
	if err != nil {
		return errors.Wrap(err, "couldn't print deployments data")
	}

	currentDeployments, err := k.deployer.Deploy(ctx, sub, k.NodeDeploymentID, newDeployments)
	if err := k.updateCurrentDeployments(ctx, sub, currentDeployments); err != nil {
		log.Printf("error updating current deployments state: %s\n", err)
	}
	return err
}

// Cancel cancels all the deployments
func (k *NetworkDeployer) Cancel(ctx context.Context, sub subi.SubstrateExt) error {
	newDeployments := make(map[uint32]gridtypes.Deployment)

	currentDeployments, err := k.deployer.Deploy(ctx, sub, k.NodeDeploymentID, newDeployments)
	if err := k.updateCurrentDeployments(ctx, sub, currentDeployments); err != nil {
		log.Printf("error updating current deployments: %s\n", err)
	}
	return err
}

// Stage for staging workloads
func (k *NetworkDeployer) Stage(ctx context.Context) error {
	// TODO: to be copied to deployer manager, or maybe not needed
	_, netWorkloads, err := k.GenerateVersionlessDeploymentsAndWorkloads(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't generate workloads data")
	}

	err = k.TFPluginClient.manager.SetWorkloads(netWorkloads)
	if err != nil {
		return errors.Wrap(err, "couldn't set workloads data")
	}

	return nil
}

// invalidateBrokenAttributes removes outdated attrs and deleted contracts
func (k *NetworkDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt) error {

	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, substrate.ErrNotFound) {
			delete(k.NodeDeploymentID, node)
			delete(k.NodesIPRange, node)
			delete(k.Keys, node)
			delete(k.WGPort, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		}
	}
	if k.ExternalIP != nil && !k.ZNet.IPRange.Contains(k.ExternalIP.IP) {
		k.ExternalIP = nil
	}
	for node, ip := range k.NodesIPRange {
		if !k.ZNet.IPRange.Contains(ip.IP) {
			delete(k.NodesIPRange, node)
		}
	}
	if k.PublicNodeID != 0 {
		// TODO: add a check that the node is still public
		cl, err := k.ncPool.GetNodeClient(sub, k.PublicNodeID)
		if err != nil {
			// whatever the error, delete it and it will get reassigned later
			k.PublicNodeID = 0
		}
		if err := cl.IsNodeUp(context.Background()); err != nil {
			k.PublicNodeID = 0
		}
	}

	if !k.ZNet.AddWGAccess {
		k.ExternalIP = nil
	}
	return nil
}

func (k *NetworkDeployer) updateCurrentDeployments(ctx context.Context, sub subi.SubstrateExt, currentDeploymentIDs map[uint32]uint64) error {
	k.NodeDeploymentID = currentDeploymentIDs
	if err := k.readNodesConfig(ctx, sub); err != nil {
		return errors.Wrap(err, "couldn't read node's data")
	}

	return nil
}

func (k *NetworkDeployer) assignNodesIPs(nodes []uint32) error {
	ips := make(map[uint32]gridtypes.IPNet)
	l := len(k.ZNet.IPRange.IP)
	usedIPs := make([]byte, 0) // the third octet
	for node, ip := range k.NodesIPRange {
		if workloads.Contains(nodes, node) {
			usedIPs = append(usedIPs, ip.IP[l-2])
			ips[node] = ip
		}
	}
	var cur byte = 2
	if k.ZNet.AddWGAccess {
		if k.ExternalIP != nil {
			usedIPs = append(usedIPs, k.ExternalIP.IP[l-2])
		} else {
			err := workloads.NextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ip := workloads.IPNet(k.ZNet.IPRange.IP[l-4], k.ZNet.IPRange.IP[l-3], cur, k.ZNet.IPRange.IP[l-1], 24)
			k.ExternalIP = &ip
		}
	}
	for _, nodeID := range nodes {
		if _, ok := ips[nodeID]; !ok {
			err := workloads.NextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ips[nodeID] = workloads.IPNet(k.ZNet.IPRange.IP[l-4], k.ZNet.IPRange.IP[l-3], cur, k.ZNet.IPRange.IP[l-2], 24)
		}
	}
	k.NodesIPRange = ips
	return nil
}

func (k *NetworkDeployer) assignNodesWGPort(ctx context.Context, sub subi.SubstrateExt, nodes []uint32) error {
	for _, nodeID := range nodes {
		if _, ok := k.WGPort[nodeID]; !ok {
			cl, err := k.ncPool.GetNodeClient(sub, nodeID)
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

func (k *NetworkDeployer) readNodesConfig(ctx context.Context, sub subi.SubstrateExt) error {
	keys := make(map[uint32]wgtypes.Key)
	WGPort := make(map[uint32]int)
	nodesIPRange := make(map[uint32]gridtypes.IPNet)
	log.Printf("reading node config")
	nodeDeployments, err := k.deployer.GetDeployments(ctx, sub, k.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "failed to get deployment objects")
	}
	err = deployer.PrintDeployments(nodeDeployments)
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
	k.NodesIPRange = nodesIPRange
	k.ZNet.AddWGAccess = WGAccess
	if !WGAccess {
		k.AccessWGConfig = ""
	}
	return nil
}
