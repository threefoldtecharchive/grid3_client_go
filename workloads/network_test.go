// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// Network
var Network = ZNet{
	Name:        "testingNetwork",
	Description: "network for testing",
	Nodes:       []uint32{1},
	IPRange: gridtypes.NewIPNet(net.IPNet{
		IP:   net.IPv4(10, 20, 0, 0),
		Mask: net.CIDRMask(16, 32),
	}),
	AddWGAccess: false,
}

func TestNetwork(t *testing.T) {
	gridProxyClient := proxy.NewRetryingClient(proxy.NewClient("https://gridproxy.dev.grid.tf/"))
	publicNode := uint32(14)

	t.Run("test_ip_net", func(t *testing.T) {
		ip := IPNet(10, 20, 0, 0, 16)
		assert.Equal(t, ip, Network.IPRange)
	})

	t.Run("test_wg_ip", func(t *testing.T) {
		wgIP := WgIP(Network.IPRange)

		wgIPRange, err := gridtypes.ParseIPNet("100.64.20.0/32")
		assert.NoError(t, err)

		assert.Equal(t, wgIP, wgIPRange)
	})

	t.Run("test_generate_wg_config", func(t *testing.T) {
		config := GenerateWGConfig(
			"", "", "", "",
			Network.IPRange.String(),
		)

		assert.Equal(t, config, strings.ReplaceAll(fmt.Sprintf(`
			[Interface]
			Address = %s
			PrivateKey = %s
			[Peer]
			PublicKey = %s
			AllowedIPs = %s, 100.64.0.0/16
			PersistentKeepalive = 25
			Endpoint = %s
			`, "", "", "", Network.IPRange.String(), ""), "\t", "")+"\t",
		)
	})

	t.Run("test_get_public_node", func(t *testing.T) {
		nodeID, err := GetPublicNode(
			context.Background(),
			gridProxyClient,
			[]uint32{},
		)
		assert.NoError(t, err)
		assert.Equal(t, nodeID, publicNode)

	})
}
