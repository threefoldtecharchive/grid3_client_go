// package deployer for grid deployer
package deployer

import (
	"context"
	"fmt"
	"log"
	"math/big"

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
	Name        string
	Description string
	Nodes       []uint32
	IPRange     gridtypes.IPNet
	AddWGAccess bool

	AccessWGConfig   string
	ExternalIP       *gridtypes.IPNet
	ExternalSK       wgtypes.Key
	ExternalPK       wgtypes.Key
	PublicNodeID     uint32
	NodeDeploymentID map[uint32]uint64
	NodesIPRange     map[uint32]gridtypes.IPNet

	WGPort map[uint32]int
	Keys   map[uint32]wgtypes.Key
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(manager DeploymentManager, network workloads.ZNet) (NetworkDeployer, error) {
	// externalIP, err := gridtypes.ParseIPNet(userAccess.UserAddress)
	// if err != nil {
	// 	return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse user address")
	// }
	// var secretKey wgtypes.Key
	// if userSecretKey == "" {
	// 	secretKey, err := wgtypes.GeneratePrivateKey()
	// 	if err != nil {
	// 		return NetworkDeployer{}, errors.Wrapf(err, "couldn't generate new private key")
	// 	}
	// } else {
	// 	secretKey, err := wgtypes.ParseKey(userSecretKey)
	// 	if err != nil {
	// 		return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse private key %s", userSecretKey)
	// 	}
	// }

	k := NetworkDeployer{
		Name:             network.Name,
		Description:      network.Description,
		Nodes:            network.Nodes,
		IPRange:          network.IPRange,
		AddWGAccess:      network.AddWGAccess,
		ExternalIP:       nil,
		ExternalSK:       wgtypes.Key{},
		ExternalPK:       wgtypes.Key{},
		AccessWGConfig:   "",
		PublicNodeID:     0,
		NodeDeploymentID: make(map[uint32]uint64),
		NodesIPRange:     make(map[uint32]gridtypes.IPNet),
		WGPort:           make(map[uint32]int),
		Keys:             make(map[uint32]wgtypes.Key),
	}

	peerSubnets := map[string]*zos.Peer{}
	nodeSubnets := map[string]bool{}
	// retrieve last network state
	oldDeployments := map[uint32]uint64{}
	for k, v := range manager.GetContractIDs() {
		oldDeployments[k] = v
	}
	for _, nodeID := range k.Nodes {

		if _, ok := oldDeployments[nodeID]; !ok {
			// if node is new, it has no previous state and shouldn't be processed
			continue
		}
		delete(oldDeployments, nodeID)

		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment with nodeID %d", nodeID)
		}
		for _, wl := range dl.Workloads {
			if wl.Name.String() == network.Name && wl.Result.State == gridtypes.StateOk {
				dataI, err := wl.WorkloadData()
				if err != nil {
					return NetworkDeployer{}, errors.Wrapf(err, "couldn't get workload \"%s\" data", wl.Name.String())
				}
				data, ok := dataI.(*zos.Network)
				if !ok {
					return NetworkDeployer{}, errors.New("couldn't cast workload data")
				}
				privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
				if err != nil {
					return NetworkDeployer{}, errors.Wrap(err, "couldn't parse private key")
				}
				for idx, peer := range data.Peers {
					peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
					if k.AddWGAccess && peer.Endpoint == "" {
						// this is the access node
						k.PublicNodeID = nodeID
					}
				}
				nodeSubnets[data.Subnet.String()] = true

				k.NodeDeploymentID[nodeID] = dl.ContractID
				k.Keys[nodeID] = privateKey
				k.WGPort[nodeID] = int(data.WGListenPort)
				k.NodesIPRange[nodeID] = data.Subnet
			}
		}

	}
	// TODO: if oldDeployments is not empty and has any of this networks' workloads, they should be canceled
	// if the workload represents an access node, and the user requires wg access, the workload should not be cancelled
	toCancel := map[uint32]map[string]bool{}
	for nodeID, contractID := range oldDeployments {
		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment %d", contractID)
		}
		wlID, err := dl.Get(gridtypes.Name(network.Name))

		if err == nil {
			wl := *(wlID.Workload)
			dataI, err := wl.WorkloadData()
			if err != nil {
				return NetworkDeployer{}, errors.Wrap(err, "failed to get workload data")
			}
			data, ok := dataI.(*zos.Network)
			if !ok {
				return NetworkDeployer{}, errors.New("couldn't cast workload data")
			}
			privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrap(err, "couldn't parse private key")
			}

			for idx, peer := range data.Peers {
				peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
				if k.AddWGAccess && peer.Endpoint == "" {
					// this is the access node
					k.PublicNodeID = nodeID
					k.NodeDeploymentID[nodeID] = dl.ContractID
					k.Keys[nodeID] = privateKey
					k.WGPort[nodeID] = int(data.WGListenPort)
					k.NodesIPRange[nodeID] = data.Subnet
					break
				}
			}
			nodeSubnets[data.Subnet.String()] = true

			if !k.AddWGAccess || k.PublicNodeID != nodeID {
				// this is a node to be cancelled
				toCancel[nodeID] = make(map[string]bool)
				toCancel[nodeID][network.Name] = true
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
			k.ExternalIP = &externalIP
			pk, err := wgtypes.ParseKey(peer.WGPublicKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse peer wg public key")
			}
			k.ExternalPK = pk
			break
		}
	}

	if k.ExternalIP == nil {
		// user does not have userAccess configs and a new private key should be generated
		secretKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't generate new private key")
		}
		k.ExternalSK = secretKey
		k.ExternalPK = secretKey.PublicKey()
	}

	err := manager.CancelWorkloads(toCancel)
	if err != nil {
		return NetworkDeployer{}, errors.Wrapf(err, "couldn't cancel workloads")
	}

	return k, nil
}

func nextFreeOctet(used []byte, start *byte) error {
	for workloads.Contains(used, *start) && *start <= 254 {
		*start++
	}
	if *start == 255 {
		return errors.New("couldn't find a free ip to add node")
	}
	return nil
}

func (k *NetworkDeployer) assignNodesIPs(nodes []uint32) error {
	ips := make(map[uint32]gridtypes.IPNet)
	l := len(k.IPRange.IP)
	usedIPs := make([]byte, 0) // the third octet
	for node, ip := range k.NodesIPRange {
		if workloads.Contains(nodes, node) {
			usedIPs = append(usedIPs, ip.IP[l-2])
			ips[node] = ip
		}
	}
	var cur byte = 2
	if k.AddWGAccess {
		if k.ExternalIP != nil {
			usedIPs = append(usedIPs, k.ExternalIP.IP[l-2])
		} else {
			err := nextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ip := workloads.IPNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-1], 24)
			k.ExternalIP = &ip
		}
	}
	for _, node := range nodes {
		if _, ok := ips[node]; !ok {
			err := nextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ips[node] = workloads.IPNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-2], 24)
		}
	}
	k.NodesIPRange = ips
	return nil
}
func (k *NetworkDeployer) assignNodesWGPort(ctx context.Context, sub subi.SubstrateExt, nodes []uint32, ncPool *client.NodeClientPool) error {
	for _, node := range nodes {
		if _, ok := k.WGPort[node]; !ok {
			cl, err := ncPool.GetNodeClient(sub, node)
			if err != nil {
				return errors.Wrap(err, "couldn't get node client")
			}
			port, err := workloads.GetNodeFreeWGPort(ctx, cl, node)
			if err != nil {
				return errors.Wrap(err, "failed to get node free wg ports")
			}
			k.WGPort[node] = port
		}
	}

	return nil
}
func (k *NetworkDeployer) assignNodesWGKey(nodes []uint32) error {
	for _, node := range nodes {
		if _, ok := k.Keys[node]; !ok {

			key, err := wgtypes.GenerateKey()
			if err != nil {
				return errors.Wrap(err, "failed to generate wg private key")
			}
			k.Keys[node] = key
		}
	}

	return nil
}

// Validate validates a network deployer
func (k *NetworkDeployer) Validate(ctx context.Context, sub subi.SubstrateExt, identity subi.Identity, ncPool *client.NodeClientPool) error {
	if err := validateAccountBalanceForExtrinsics(sub, identity); err != nil {
		return err
	}
	mask := k.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in ip range %s should be 16", k.IPRange.String())
	}

	return client.AreNodesUp(ctx, sub, k.Nodes, ncPool)
}

func validateAccountBalanceForExtrinsics(sub subi.SubstrateExt, identity subi.Identity) error {
	acc, err := sub.GetAccount(identity)
	if err != nil && !errors.Is(err, substrate.ErrAccountNotFound) {
		return errors.Wrap(err, "failed to get account with the given mnemonics")
	}
	log.Printf("money %d\n", acc.Data.Free)
	if acc.Data.Free.Cmp(big.NewInt(20000)) == -1 {
		return fmt.Errorf("account workloads.workloads.Contains %s, min fee is 20000", acc.Data.Free)
	}
	return nil
}

func (k *NetworkDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt, ncPool *client.NodeClientPool) error {

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
	if k.ExternalIP != nil && !k.IPRange.Contains(k.ExternalIP.IP) {
		k.ExternalIP = nil
	}
	for node, ip := range k.NodesIPRange {
		if !k.IPRange.Contains(ip.IP) {
			delete(k.NodesIPRange, node)
		}
	}
	if k.PublicNodeID != 0 {
		// TODO: add a check that the node is still public
		cl, err := ncPool.GetNodeClient(sub, k.PublicNodeID)
		if err != nil {
			// whatever the error, delete it and it will get reassigned later
			k.PublicNodeID = 0
		}
		if err := cl.IsNodeUp(context.Background()); err != nil {
			k.PublicNodeID = 0
		}
	}

	if !k.AddWGAccess {
		k.ExternalIP = nil
	}
	return nil
}

// Stage for staging workloads
func (k *NetworkDeployer) Stage(ctx context.Context, apiClient APIClient, znet workloads.ZNet) (workloads.UserAccess, error) {
	// TODO: to be copied to deployer manager, or maybe not needed
	// err := k.Validate(ctx, sub, identity, ncPool)
	// if err != nil {
	// 	return err
	// }
	userAccess := workloads.UserAccess{}
	err := k.invalidateBrokenAttributes(apiClient.SubstrateExt, apiClient.NCPool)
	if err != nil {
		return workloads.UserAccess{}, err
	}

	log.Printf("nodes: %v\n", k.Nodes)
	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	var ipv4Node uint32
	accessibleNodes := make([]uint32, 0)
	for _, node := range k.Nodes {
		cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, node)
		if err != nil {
			return workloads.UserAccess{}, errors.Wrapf(err, "couldn't get node %d client", node)
		}
		endpoint, err := workloads.GetNodeEndpoint(ctx, cl)
		if errors.Is(err, workloads.ErrNoAccessibleInterfaceFound) {
			hiddenNodes = append(hiddenNodes, node)
		} else if err != nil {
			return workloads.UserAccess{}, errors.Wrapf(err, "failed to get node %d endpoint", node)
		} else if endpoint.To4() != nil {
			accessibleNodes = append(accessibleNodes, node)
			ipv4Node = node
			endpoints[node] = endpoint.String()
		} else {
			accessibleNodes = append(accessibleNodes, node)
			endpoints[node] = fmt.Sprintf("[%s]", endpoint.String())
		}
	}
	needsIPv4Access := k.AddWGAccess || (len(hiddenNodes) != 0 && len(hiddenNodes)+len(accessibleNodes) > 1)
	if needsIPv4Access {
		if k.PublicNodeID != 0 { // it's set
			accessibleNodes = append(accessibleNodes, k.PublicNodeID)
		} else if ipv4Node != 0 { // there's one in the network original nodes
			k.PublicNodeID = ipv4Node
		} else {
			publicNode, err := workloads.GetPublicNode(ctx, apiClient.ProxyClient, []uint32{})
			if err != nil {
				return workloads.UserAccess{}, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			k.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}
		if endpoints[k.PublicNodeID] == "" { // old or new outsider
			cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, k.PublicNodeID)
			if err != nil {
				return workloads.UserAccess{}, errors.Wrapf(err, "couldn't get node %d client", k.PublicNodeID)
			}
			endpoint, err := workloads.GetNodeEndpoint(ctx, cl)
			if err != nil {
				return workloads.UserAccess{}, errors.Wrapf(err, "failed to get node %d endpoint", k.PublicNodeID)
			}
			endpoints[k.PublicNodeID] = endpoint.String()
		}
	}
	all := append(hiddenNodes, accessibleNodes...)
	if err := k.assignNodesIPs(all); err != nil {
		return workloads.UserAccess{}, errors.Wrap(err, "couldn't assign node ips")
	}
	if err := k.assignNodesWGKey(all); err != nil {
		return workloads.UserAccess{}, errors.Wrap(err, "couldn't assign node wg keys")
	}
	if err := k.assignNodesWGPort(ctx, apiClient.SubstrateExt, all, apiClient.NCPool); err != nil {
		return workloads.UserAccess{}, errors.Wrap(err, "couldn't assign node wg ports")
	}
	nonAccessibleIPRanges := []gridtypes.IPNet{}
	for _, node := range hiddenNodes {
		r := k.NodesIPRange[node]
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(r))
	}
	if k.AddWGAccess {
		r := k.ExternalIP
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, *r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, workloads.WgIP(*r))
	}
	log.Printf("hidden nodes: %v\n", hiddenNodes)
	log.Printf("public node: %v\n", k.PublicNodeID)
	log.Printf("accessible nodes: %v\n", accessibleNodes)
	log.Printf("non accessible ip ranges: %v\n", nonAccessibleIPRanges)

	// returning wg user access should only happen if k.ExternalSK is set
	emptyKey := wgtypes.Key{}
	if k.AddWGAccess && k.ExternalSK != emptyKey {

		userAccess.UserAddress = k.ExternalIP.String()
		userAccess.UserSecretKey = k.ExternalSK.String()
		userAccess.PublicNodePK = k.Keys[k.PublicNodeID].PublicKey().String()
		userAccess.AllowedIPs = []string{k.IPRange.String(), "100.64.0.0/16"}
		userAccess.PublicNodeEndpoint = fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID])
	}

	netWorkloads := map[uint32][]gridtypes.Workload{}

	for _, node := range accessibleNodes {
		peers := make([]zos.Peer, 0, len(k.Nodes))
		for _, neigh := range accessibleNodes {
			if neigh == node {
				continue
			}
			neighIPRange := k.NodesIPRange[neigh]
			allowedIPs := []gridtypes.IPNet{
				neighIPRange,
				workloads.WgIP(neighIPRange),
			}
			if neigh == k.PublicNodeID {
				allowedIPs = append(allowedIPs, nonAccessibleIPRanges...)
			}
			peers = append(peers, zos.Peer{
				Subnet:      k.NodesIPRange[neigh],
				WGPublicKey: k.Keys[neigh].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[neigh], k.WGPort[neigh]),
				AllowedIPs:  allowedIPs,
			})
		}
		if node == k.PublicNodeID {
			// external node
			if k.AddWGAccess {
				peers = append(peers, zos.Peer{
					Subnet:      *k.ExternalIP,
					WGPublicKey: k.ExternalPK.String(),
					AllowedIPs:  []gridtypes.IPNet{*k.ExternalIP, workloads.WgIP(*k.ExternalIP)},
				})
			}
			// hidden nodes
			for _, neigh := range hiddenNodes {
				neighIPRange := k.NodesIPRange[neigh]
				peers = append(peers, zos.Peer{
					Subnet:      neighIPRange,
					WGPublicKey: k.Keys[neigh].PublicKey().String(),
					AllowedIPs: []gridtypes.IPNet{
						neighIPRange,
						workloads.WgIP(neighIPRange),
					},
				})
			}
		}

		workload := gridtypes.Workload{
			Version:     0,
			Type:        zos.NetworkType,
			Description: k.Description,
			Name:        gridtypes.Name(k.Name),
			Data: gridtypes.MustMarshal(zos.Network{
				NetworkIPRange: gridtypes.MustParseIPNet(k.IPRange.String()),
				Subnet:         k.NodesIPRange[node],
				WGPrivateKey:   k.Keys[node].String(),
				WGListenPort:   uint16(k.WGPort[node]),
				Peers:          peers,
			}),
		}
		netWorkloads[node] = append(netWorkloads[node], workload)
	}
	// hidden nodes deployments
	for _, node := range hiddenNodes {
		nodeIPRange := k.NodesIPRange[node]
		peers := make([]zos.Peer, 0)
		if k.PublicNodeID != 0 {
			peers = append(peers, zos.Peer{
				WGPublicKey: k.Keys[k.PublicNodeID].PublicKey().String(),
				Subnet:      nodeIPRange,
				AllowedIPs: []gridtypes.IPNet{
					k.IPRange,
					workloads.IPNet(100, 64, 0, 0, 16),
				},
				Endpoint: fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID]),
			})
		}
		workload := gridtypes.Workload{
			Version:     0,
			Type:        zos.NetworkType,
			Description: k.Description,
			Name:        gridtypes.Name(k.Name),
			Data: gridtypes.MustMarshal(zos.Network{
				NetworkIPRange: gridtypes.MustParseIPNet(k.IPRange.String()),
				Subnet:         nodeIPRange,
				WGPrivateKey:   k.Keys[node].String(),
				WGListenPort:   uint16(k.WGPort[node]),
				Peers:          peers,
			}),
		}
		netWorkloads[node] = append(netWorkloads[node], workload)
	}

	err = apiClient.Manager.SetWorkloads(netWorkloads)
	if err != nil {
		return workloads.UserAccess{}, errors.Wrap(err, "couldn't ")
	}

	return userAccess, nil
}
