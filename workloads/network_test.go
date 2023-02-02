// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestNetwork(t *testing.T) {
	gridProxyClient := proxy.NewRetryingClient(proxy.NewClient("https://gridproxy.dev.grid.tf/"))

	ipRange, err := gridtypes.ParseIPNet("1.1.1.1/24")
	assert.NoError(t, err)

	znet := ZNet{
		Name:        "test",
		Description: "test description",
		Nodes:       []uint32{1},
		IPRange:     ipRange,
		AddWGAccess: true,
	}

	t.Run("test_ip_net", func(t *testing.T) {
		ip := IPNet(1, 1, 1, 1, 24)
		assert.Equal(t, ip, znet.IPRange)
	})

	t.Run("test_wg_ip", func(t *testing.T) {
		wgIP := WgIP(znet.IPRange)

		wgIPRange, err := gridtypes.ParseIPNet("100.64.1.1/32")
		assert.NoError(t, err)

		assert.Equal(t, wgIP, wgIPRange)
	})

	t.Run("test_generate_wg_config", func(t *testing.T) {
		GenerateWGConfig(
			"", "", "", "",
			znet.IPRange.String(),
		)
	})

	t.Run("test_get_public_node", func(t *testing.T) {
		nodeID, err := GetPublicNode(
			context.Background(),
			gridProxyClient,
			[]uint32{},
		)
		assert.NoError(t, err)
		assert.Equal(t, nodeID, uint32(14))

	})
}
