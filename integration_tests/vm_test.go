// Package integration for integration tests
package integration

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestVMDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, _, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := deployer.NodeFilter{
		CRU:       2,
		SRU:       2,
		MRU:       1,
		Status:    "up",
		PublicIPs: true,
	}
	nodeIDs, err := deployer.FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	nodeID := nodeIDs[0]

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		CPU:        2,
		PublicIP:   true,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		IP:          "10.20.2.5",
		NetworkName: network.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("check single vm with public ip", func(t *testing.T) {
		err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
		assert.NoError(t, err)

		dl := workloads.NewDeployment("vm", nodeID, "", nil, network.Name, nil, nil, []workloads.VM{vm}, nil)
		err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
		assert.NoError(t, err)

		v, err := tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm.Name)
		assert.NoError(t, err)
		assert.Equal(t, v.IP, "10.20.2.5")

		publicIP := strings.Split(v.ComputedIP, "/")[0]
		assert.NotEmpty(t, publicIP)
		if !TestConnection(publicIP, "22") {
			t.Errorf("public ip is not reachable")
		}

		yggIP := v.YggIP
		assert.NotEmpty(t, yggIP)
		if !TestConnection(yggIP, "22") {
			t.Errorf("yggdrasil ip is not reachable")
		}

		// cancel all
		err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
		assert.NoError(t, err)

		err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
		assert.NoError(t, err)

		_, err = tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm.Name)
		assert.Error(t, err)
	})
}
