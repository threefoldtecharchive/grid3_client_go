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

	// computed
	SolutionType     string
	AccessWGConfig   string
	ExternalIP       *gridtypes.IPNet
	ExternalSK       wgtypes.Key
	PublicNodeID     uint32
	NodesIPRange     map[uint32]gridtypes.IPNet
	NodeDeploymentID map[uint32]uint64
}

// NewNetworkFromWorkload generates a new znet from a workload
func NewNetworkFromWorkload(wl gridtypes.Workload, nodeID uint32) (ZNet, error) {
	data, err := GetZNetWorkloadData(wl)
	if err != nil {
		return ZNet{}, err
	}

	return ZNet{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Nodes:       []uint32{nodeID},
		IPRange:     data.NetworkIPRange,
		AddWGAccess: data.WGPrivateKey != "",
	}, nil
}

// Validate validates a network
func (znet *ZNet) Validate() error {
	mask := znet.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in ip range %s should be 16", znet.IPRange.String())
	}

	return nil
}

// GetZNetWorkloadData retrieves network workload data
func GetZNetWorkloadData(wl gridtypes.Workload) (*zos.Network, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return &zos.Network{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.Network)
	if !ok {
		return &zos.Network{}, errors.New("could not create network workload")
	}

	return data, nil
}

// GenerateWorkloads generates a workload from a znet
func (znet *ZNet) GenerateWorkload(subnet gridtypes.IPNet, wgPrivateKey string, wgListenPort uint16, peers []zos.Peer) gridtypes.Workload {
	return gridtypes.Workload{
		Version:     0,
		Type:        zos.NetworkType,
		Description: znet.Description,
		Name:        gridtypes.Name(znet.Name),
		Data: gridtypes.MustMarshal(zos.Network{
			NetworkIPRange: gridtypes.MustParseIPNet(znet.IPRange.String()),
			Subnet:         subnet,
			WGPrivateKey:   wgPrivateKey,
			WGListenPort:   wgListenPort,
			Peers:          peers,
		}),
	}
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

// NextFreeOctet finds a free ip for a node
func NextFreeOctet(used []byte, start *byte) error {
	for Contains(used, *start) && *start <= 254 {
		*start++
	}
	if *start == 255 {
		return errors.New("couldn't find a free ip to add node")
	}
	return nil
}
