// Package workloads includes workloads types (vm, zdb, QSFS, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/pkg/errors"
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
	dataI, err := wl.WorkloadData()
	if err != nil {
		return ZNet{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.Network)
	if !ok {
		return ZNet{}, fmt.Errorf("could not create network workload from data %v", dataI)
	}

	return ZNet{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Nodes:       []uint32{nodeID},
		IPRange:     data.NetworkIPRange,
		AddWGAccess: data.WGPrivateKey != "",
	}, nil
}

// Validate validates a network mask to be 16
func (znet *ZNet) Validate() error {
	mask := znet.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in ip range %s should be 16", znet.IPRange.String())
	}

	return nil
}

// ZosWorkload generates a zos workload from a network
func (znet *ZNet) ZosWorkload(subnet gridtypes.IPNet, wgPrivateKey string, wgListenPort uint16, peers []zos.Peer) gridtypes.Workload {
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

// GenerateMetadata generates deployment metadata
func (znet *ZNet) GenerateMetadata() (string, error) {
	if len(znet.SolutionType) == 0 {
		znet.SolutionType = "Network"
	}

	deploymentData := DeploymentData{
		Name:        znet.Name,
		Type:        "network",
		ProjectName: znet.SolutionType,
	}

	deploymentDataBytes, err := json.Marshal(deploymentData)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse deployment data %v", deploymentData)
	}

	return string(deploymentDataBytes), nil
}

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

// NextFreeIP finds a free ip for a node
func NextFreeIP(used []byte, start *byte) error {
	for Contains(used, *start) && *start <= 254 {
		*start++
	}
	if *start == 255 {
		return errors.New("could not find a free ip to add node")
	}
	return nil
}
