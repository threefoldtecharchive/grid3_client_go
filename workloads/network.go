package workloads

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type APIClient struct {
	SubstrateExt subi.SubstrateExt
	NCPool       *client.NodeClientPool
	ProxyClient  proxy.Client
	Manager      deployer.DeploymentManager
	Identity     subi.Identity
}
type UserAccess struct {
	UserAddress        *gridtypes.IPNet
	UserSecretKey      wgtypes.Key
	PublicNodePK       wgtypes.Key
	AllowedIPs         []gridtypes.IPNet
	PublicNodeEndpoint string
}
type TargetNetwork struct {
	Name        string
	Description string
	Nodes       []uint32
	IPRange     gridtypes.IPNet
	AddWGAccess bool
}
type NetworkDeployer struct {
	Name        string
	Description string
	Nodes       []uint32
	IPRange     gridtypes.IPNet
	AddWGAccess bool

	AccessWGConfig   string
	ExternalIP       *gridtypes.IPNet
	ExternalSK       wgtypes.Key
	PublicNodeID     uint32
	NodeDeploymentID map[uint32]uint64
	NodesIPRange     map[uint32]gridtypes.IPNet

	WGPort map[uint32]int
	Keys   map[uint32]wgtypes.Key
}

func nextFreeOctet(used []byte, start *byte) error {
	for isInByte(used, *start) && *start <= 254 {
		*start += 1
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
		if isInUint32(nodes, node) {
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
			ip := ipNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-1], 24)
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
			ips[node] = ipNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-2], 24)
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
				return errors.Wrap(err, "coudln't get node client")
			}
			port, err := getNodeFreeWGPort(ctx, cl, node)
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

func (k *NetworkDeployer) Validate(ctx context.Context, sub subi.SubstrateExt, identity subi.Identity, ncPool *client.NodeClientPool) error {
	if err := validateAccountMoneyForExtrinsics(sub, identity); err != nil {
		return err
	}
	mask := k.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in iprange %s should be 16", k.IPRange.String())
	}

	return isNodesUp(ctx, sub, k.Nodes, ncPool)
}

func validateAccountMoneyForExtrinsics(sub subi.SubstrateExt, identity subi.Identity) error {
	acc, err := sub.GetAccount(identity)
	if err != nil && !errors.Is(err, subi.ErrAccountNotFound) {
		return errors.Wrap(err, "failed to get account with the given mnemonics")
	}
	log.Printf("money %d\n", acc.Data.Free)
	if acc.Data.Free.Cmp(big.NewInt(20000)) == -1 {
		return fmt.Errorf("account contains %s, min fee is 20000", acc.Data.Free)
	}
	return nil
}

func (k *NetworkDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt, ncPool *client.NodeClientPool) error {

	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, subi.ErrNotFound) {
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
		if err := isNodeUp(context.Background(), cl); err != nil {
			k.PublicNodeID = 0
		}
	}

	if !k.AddWGAccess {
		k.ExternalIP = nil
	}
	return nil
}

func NewNetworkDeployer(manager deployer.DeploymentManager, userAccess *UserAccess, target TargetNetwork) (NetworkDeployer, error) {
	k := NetworkDeployer{
		Name:        target.Name,
		Description: target.Description,
		Nodes:       target.Nodes,
		IPRange:     target.IPRange,
		AddWGAccess: target.AddWGAccess,
		ExternalIP:  userAccess.UserAddress,
		ExternalSK:  userAccess.UserSecretKey,
	}
	if k.ExternalSK.String() == "" {
		secretKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return NetworkDeployer{}, errors.Wrap(err, "couldn't generate new secret key")
		}
		k.ExternalSK = secretKey
	}
	for _, nodeID := range k.Nodes {
		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrap(err, "couldn't build newtork deployer")
		}
		for _, wl := range dl.Workloads {
			if wl.Name.String() == target.Name {
				dataI, err := wl.WorkloadData()
				if err != nil {
					return NetworkDeployer{}, errors.Wrap(err, "couldn't build newtork deployer")
				}
				data, ok := dataI.(*zos.Network)
				if !ok {
					return NetworkDeployer{}, errors.New("couldn't cast workload data")
				}
				privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
				if err != nil {
					return NetworkDeployer{}, errors.Wrap(err, "couldn't build newtork deployer")
				}
				if privateKey.PublicKey() == userAccess.PublicNodePK {
					// this is the access node
					k.PublicNodeID = nodeID
				}
				k.NodeDeploymentID[nodeID] = dl.ContractID
				k.Keys[nodeID] = privateKey
				k.WGPort[nodeID] = int(data.WGListenPort)
				k.NodesIPRange[nodeID] = data.Subnet
			}
		}
	}
	return k, nil
}

func (network *TargetNetwork) Stage(
	ctx context.Context,
	apiClient APIClient,
	userAccess *UserAccess) error {
	// TODO: to be copied to deployer manager, or maybe not needed
	// err := k.Validate(ctx, sub, identity, ncPool)
	// if err != nil {
	// 	return err
	// }
	k, err := NewNetworkDeployer(apiClient.Manager, userAccess, *network)
	if err != nil {
		return errors.Wrap(err, "couldn't build network deployer")
	}

	err = k.invalidateBrokenAttributes(apiClient.SubstrateExt, apiClient.NCPool)
	if err != nil {
		return err
	}
	log.Printf("nodes: %v\n", k.Nodes)
	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	var ipv4Node uint32
	accessibleNodes := make([]uint32, 0)
	for _, node := range k.Nodes {
		cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, node)
		if err != nil {
			return errors.Wrapf(err, "couldn't get node %d client", node)
		}
		endpoint, err := getNodeEndpoint(ctx, cl)
		if errors.Is(err, ErrNoAccessibleInterfaceFound) {
			hiddenNodes = append(hiddenNodes, node)
		} else if err != nil {
			return errors.Wrapf(err, "failed to get node %d endpoint", node)
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
		} else if ipv4Node != 0 { // there's one in the network original nodes
			k.PublicNodeID = ipv4Node
		} else {
			publicNode, err := getPublicNode(ctx, apiClient.ProxyClient, []uint32{})
			if err != nil {
				return errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			k.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}
		if endpoints[k.PublicNodeID] == "" { // old or new outsider
			cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, k.PublicNodeID)
			if err != nil {
				return errors.Wrapf(err, "couldn't get node %d client", k.PublicNodeID)
			}
			endpoint, err := getNodeEndpoint(ctx, cl)
			if err != nil {
				return errors.Wrapf(err, "failed to get node %d endpoint", k.PublicNodeID)
			}
			endpoints[k.PublicNodeID] = endpoint.String()
		}
	}
	all := append(hiddenNodes, accessibleNodes...)
	if err := k.assignNodesIPs(all); err != nil {
		return errors.Wrap(err, "couldn't assign node ips")
	}
	if err := k.assignNodesWGKey(all); err != nil {
		return errors.Wrap(err, "couldn't assign node wg keys")
	}
	if err := k.assignNodesWGPort(ctx, apiClient.SubstrateExt, all, apiClient.NCPool); err != nil {
		return errors.Wrap(err, "couldn't assign node wg ports")
	}
	nonAccessibleIPRanges := []gridtypes.IPNet{}
	for _, node := range hiddenNodes {
		r := k.NodesIPRange[node]
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, wgIP(r))
	}
	if k.AddWGAccess {
		r := k.ExternalIP
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, *r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, wgIP(*r))
	}
	log.Printf("hidden nodes: %v\n", hiddenNodes)
	log.Printf("public node: %v\n", k.PublicNodeID)
	log.Printf("accessible nodes: %v\n", accessibleNodes)
	log.Printf("non accessible ip ranges: %v\n", nonAccessibleIPRanges)

	if k.AddWGAccess {
		k.AccessWGConfig = generateWGConfig(
			wgIP(*k.ExternalIP).IP.String(),
			k.ExternalSK.String(),
			k.Keys[k.PublicNodeID].PublicKey().String(),
			fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID]),
			k.IPRange.String(),
		)
		userAccess = &UserAccess{
			UserAddress:        k.ExternalIP,
			UserSecretKey:      k.ExternalSK,
			PublicNodePK:       k.Keys[k.PublicNodeID].PublicKey(),
			AllowedIPs:         []gridtypes.IPNet{k.IPRange, ipNet(100, 64, 0, 0, 16)},
			PublicNodeEndpoint: fmt.Sprintf("%s:%d", endpoints[k.PublicNodeID], k.WGPort[k.PublicNodeID]),
		}
	}
	workloads := map[uint32][]gridtypes.Workload{}

	for _, node := range accessibleNodes {
		peers := make([]zos.Peer, 0, len(k.Nodes))
		for _, neigh := range accessibleNodes {
			if neigh == node {
				continue
			}
			neighIPRange := k.NodesIPRange[neigh]
			allowed_ips := []gridtypes.IPNet{
				neighIPRange,
				wgIP(neighIPRange),
			}
			if neigh == k.PublicNodeID {
				allowed_ips = append(allowed_ips, nonAccessibleIPRanges...)
			}
			peers = append(peers, zos.Peer{
				Subnet:      k.NodesIPRange[neigh],
				WGPublicKey: k.Keys[neigh].PublicKey().String(),
				Endpoint:    fmt.Sprintf("%s:%d", endpoints[neigh], k.WGPort[neigh]),
				AllowedIPs:  allowed_ips,
			})
		}
		if node == k.PublicNodeID {
			// external node
			if k.AddWGAccess {
				peers = append(peers, zos.Peer{
					Subnet:      *k.ExternalIP,
					WGPublicKey: k.ExternalSK.PublicKey().String(),
					AllowedIPs:  []gridtypes.IPNet{*k.ExternalIP, wgIP(*k.ExternalIP)},
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
						wgIP(neighIPRange),
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
		workloads[node] = append(workloads[node], workload)
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
					ipNet(100, 64, 0, 0, 16),
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
		workloads[node] = append(workloads[node], workload)
	}

	err = apiClient.Manager.SetWorkloads(workloads)
	if err != nil {
		return errors.Wrap(err, "couldn't ")
	}
	return nil
}
