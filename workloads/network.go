// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/todo"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	proxyTypes "github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// UserAccess struct
type UserAccess struct {
	UserAddress        string
	UserSecretKey      string
	PublicNodePK       string
	AllowedIPs         []string
	PublicNodeEndpoint string
}

// ZNet is zos network workload
type ZNet struct {
	Name        string
	Description string
	Nodes       []uint32
	IPRange     gridtypes.IPNet
	AddWGAccess bool
}

var (
	trueVal  = true
	statusUp = "up"

	// ErrNoAccessibleInterfaceFound no accessible interface found
	ErrNoAccessibleInterfaceFound = fmt.Errorf("couldn't find a publicly accessible ipv4 or ipv6")
)

// IPNet returns an IP net type
func IPNet(a, b, c, d, msk byte) gridtypes.IPNet {
	return gridtypes.NewIPNet(net.IPNet{
		IP:   net.IPv4(a, b, c, d),
		Mask: net.CIDRMask(int(msk), 32),
	})
}

// WgIP return wireguard IP network
func WgIP(ip gridtypes.IPNet) gridtypes.IPNet {
	a := ip.IP[len(ip.IP)-3]
	b := ip.IP[len(ip.IP)-2]

	return gridtypes.NewIPNet(net.IPNet{
		IP:   net.IPv4(100, 64, a, b),
		Mask: net.CIDRMask(32, 32),
	})

}

// GenerateWGConfig generates wireguard configs
func GenerateWGConfig(Address string, AccessPrivatekey string, NodePublicKey string, NodeEndpoint string, NetworkIPRange string) string {

	return fmt.Sprintf(`
[Interface]
Address = %s
PrivateKey = %s
[Peer]
PublicKey = %s
AllowedIPs = %s, 100.64.0.0/16
PersistentKeepalive = 25
Endpoint = %s
	`, Address, AccessPrivatekey, NodePublicKey, NetworkIPRange, NodeEndpoint)
}

// GetPublicNode return public node ID
func GetPublicNode(ctx context.Context, gridClient proxy.Client, preferredNodes []uint32) (uint32, error) {
	preferredNodesSet := make(map[int]struct{})
	for _, node := range preferredNodes {
		preferredNodesSet[int(node)] = struct{}{}
	}
	nodes, _, err := gridClient.Nodes(proxyTypes.NodeFilter{
		IPv4:   &trueVal,
		Status: &statusUp,
	}, proxyTypes.Limit{})
	if err != nil {
		return 0, errors.Wrap(err, "couldn't fetch nodes from the rmb proxy")
	}
	// force add preferred nodes
	nodeMap := make(map[int]struct{})
	for _, node := range nodes {
		nodeMap[node.NodeID] = struct{}{}
	}
	for _, node := range preferredNodes {
		if _, ok := nodeMap[int(node)]; ok {
			continue
		}
		nodeInfo, err := gridClient.Node(node)
		if err != nil {
			log.Printf("failed to get node %d from the grid proxy", node)
			continue
		}
		if nodeInfo.PublicConfig.Ipv4 == "" {
			continue
		}
		if nodeInfo.Status != "up" {
			continue
		}
		nodes = append(nodes, proxyTypes.Node{
			PublicConfig: nodeInfo.PublicConfig,
		})
	}
	lastPreferred := 0
	for i := range nodes {
		if _, ok := preferredNodesSet[nodes[i].NodeID]; ok {
			nodes[i], nodes[lastPreferred] = nodes[lastPreferred], nodes[i]
			lastPreferred++
		}
	}
	for _, node := range nodes {
		log.Printf("found a node with ipv4 public config: %d %s\n", node.NodeID, node.PublicConfig.Ipv4)
		ip, _, err := net.ParseCIDR(node.PublicConfig.Ipv4)
		if err != nil {
			log.Printf("couldn't parse public ip %s of node %d: %s", node.PublicConfig.Ipv4, node.NodeID, err.Error())
			continue
		}
		if ip.IsPrivate() {
			log.Printf("public ip %s of node %d is private", node.PublicConfig.Ipv4, node.NodeID)
			continue
		}
		return uint32(node.NodeID), nil
	}
	return 0, errors.New("no nodes with public ipv4")
}

// GetNodeFreeWGPort returns node free wireguard port
func GetNodeFreeWGPort(ctx context.Context, nodeClient *client.NodeClient, nodeID uint32) (int, error) {
	rand.Seed(time.Now().UnixNano())
	freePorts, err := nodeClient.NetworkListWGPorts(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to list wg ports")
	}
	log.Printf("reserved ports for node %d: %v\n", nodeID, freePorts)
	p := uint(rand.Intn(6000) + 2000)

	for Contains(freePorts, uint16(p)) {
		p = uint(rand.Intn(6000) + 2000)
	}
	log.Printf("Selected port for node %d is %d\n", nodeID, p)
	return int(p), nil
}

// GetNodeEndpoint gets node end point network ip
func GetNodeEndpoint(ctx context.Context, nodeClient *client.NodeClient) (net.IP, error) {
	publicConfig, err := nodeClient.NetworkGetPublicConfig(ctx)
	log.Printf("publicConfig: %v\n", publicConfig)
	log.Printf("publicConfig.IPv4: %v\n", publicConfig.IPv4)
	log.Printf("publicConfig.IPv.IP: %v\n", publicConfig.IPv4.IP)
	log.Printf("err: %s\n", err)
	if err == nil && publicConfig.IPv4.IP != nil {

		ip := publicConfig.IPv4.IP
		log.Printf("ip: %s, global unicast: %t, privateIP: %t\n", ip.String(), ip.IsGlobalUnicast(), ip.IsPrivate())
		if ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return ip, nil
		}
	} else if err == nil && publicConfig.IPv6.IP != nil {
		ip := publicConfig.IPv6.IP
		log.Printf("ip: %s, global unicast: %t, privateIP: %t\n", ip.String(), ip.IsGlobalUnicast(), ip.IsPrivate())
		if ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return ip, nil
		}
	}

	ifs, err := nodeClient.NetworkListInterfaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list node interfaces")
	}
	log.Printf("if: %v\n", ifs)

	zosIf, ok := ifs["zos"]
	if !ok {
		return nil, errors.Wrap(ErrNoAccessibleInterfaceFound, "no zos interface")
	}
	for _, ip := range zosIf {
		log.Printf("ip: %s, global unicast: %t, privateIP: %t\n", ip.String(), ip.IsGlobalUnicast(), ip.IsPrivate())
		if !ip.IsGlobalUnicast() || ip.IsPrivate() {
			continue
		}

		return ip, nil
	}
	return nil, errors.Wrap(ErrNoAccessibleInterfaceFound, "no public ipv4 or ipv6 on zos interface found")
}

// Stage for staging workloads
func (znet *ZNet) Stage(
	ctx context.Context,
	apiClient todo.APIClient) (UserAccess, error) {
	// TODO: to be copied to deployer manager, or maybe not needed
	// err := k.Validate(ctx, sub, identity, ncPool)
	// if err != nil {
	// 	return err
	// }
	userAccess := UserAccess{}
	k, err := NewNetworkDeployer(apiClient.Manager, *znet)
	if err != nil {
		return UserAccess{}, errors.Wrap(err, "couldn't build network deployer")
	}
	err = k.invalidateBrokenAttributes(apiClient.SubstrateExt, apiClient.NCPool)
	if err != nil {
		return UserAccess{}, err
	}

	log.Printf("nodes: %v\n", k.Nodes)
	endpoints := make(map[uint32]string)
	hiddenNodes := make([]uint32, 0)
	var ipv4Node uint32
	accessibleNodes := make([]uint32, 0)
	for _, node := range k.Nodes {
		cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, node)
		if err != nil {
			return UserAccess{}, errors.Wrapf(err, "couldn't get node %d client", node)
		}
		endpoint, err := GetNodeEndpoint(ctx, cl)
		if errors.Is(err, ErrNoAccessibleInterfaceFound) {
			hiddenNodes = append(hiddenNodes, node)
		} else if err != nil {
			return UserAccess{}, errors.Wrapf(err, "failed to get node %d endpoint", node)
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
			publicNode, err := GetPublicNode(ctx, apiClient.ProxyClient, []uint32{})
			if err != nil {
				return UserAccess{}, errors.Wrap(err, "public node needed because you requested adding wg access or a hidden node is added to the network")
			}
			k.PublicNodeID = publicNode
			accessibleNodes = append(accessibleNodes, publicNode)
		}
		if endpoints[k.PublicNodeID] == "" { // old or new outsider
			cl, err := apiClient.NCPool.GetNodeClient(apiClient.SubstrateExt, k.PublicNodeID)
			if err != nil {
				return UserAccess{}, errors.Wrapf(err, "couldn't get node %d client", k.PublicNodeID)
			}
			endpoint, err := GetNodeEndpoint(ctx, cl)
			if err != nil {
				return UserAccess{}, errors.Wrapf(err, "failed to get node %d endpoint", k.PublicNodeID)
			}
			endpoints[k.PublicNodeID] = endpoint.String()
		}
	}
	all := append(hiddenNodes, accessibleNodes...)
	if err := k.assignNodesIPs(all); err != nil {
		return UserAccess{}, errors.Wrap(err, "couldn't assign node ips")
	}
	if err := k.assignNodesWGKey(all); err != nil {
		return UserAccess{}, errors.Wrap(err, "couldn't assign node wg keys")
	}
	if err := k.assignNodesWGPort(ctx, apiClient.SubstrateExt, all, apiClient.NCPool); err != nil {
		return UserAccess{}, errors.Wrap(err, "couldn't assign node wg ports")
	}
	nonAccessibleIPRanges := []gridtypes.IPNet{}
	for _, node := range hiddenNodes {
		r := k.NodesIPRange[node]
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, WgIP(r))
	}
	if k.AddWGAccess {
		r := k.ExternalIP
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, *r)
		nonAccessibleIPRanges = append(nonAccessibleIPRanges, WgIP(*r))
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
				WgIP(neighIPRange),
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
					WGPublicKey: k.ExternalPK.String(),
					AllowedIPs:  []gridtypes.IPNet{*k.ExternalIP, WgIP(*k.ExternalIP)},
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
						WgIP(neighIPRange),
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
					IPNet(100, 64, 0, 0, 16),
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
		return UserAccess{}, errors.Wrap(err, "couldn't ")
	}

	return userAccess, nil
}
