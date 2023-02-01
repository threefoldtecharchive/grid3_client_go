// Package integration for integration tests
package integration

import (
	"context"
	"strings"

	"net"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestTwoVmDifferentNet(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := NodeFilter{
		CRU:       2,
		SRU:       2,
		MRU:       1,
		Status:    "up",
		PublicIPs: true,
	}
	nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	nodeID := nodeIDs[0]

	network1 := workloads.ZNet{
		Name:        "Network1",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	network2 := workloads.ZNet{
		Name:        "Network2",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	vm1 := workloads.VM{
		Name:       "vm1",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-22.04.flist",
		CPU:        2,
		PublicIP6:  true,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		IP:          "10.1.0.2",
		NetworkName: network1.Name,
	}

	vm2 := workloads.VM{
		Name:       "vm2",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-22.04.flist",
		CPU:        2,
		PublicIP6:  true,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		IP:          "10.1.0.3",
		NetworkName: network2.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network1)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network2)
	assert.NoError(t, err)

	t.Run("check public ipv6 and yggdrasil", func(t *testing.T) {
		dl := workloads.NewDeployment("vm1", nodeID, "", nil, network1.Name, nil, nil, []workloads.VM{vm1}, nil)
		err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
		assert.NoError(t, err)

		dl = workloads.NewDeployment("vm2", nodeID, "", nil, network2.Name, nil, nil, []workloads.VM{vm2}, nil)
		err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
		assert.NoError(t, err)

		v1, err := tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm1.Name)
		assert.NoError(t, err)

		v2, err := tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm2.Name)
		assert.NoError(t, err)

		yggIP1 := v1.YggIP
		yggIP2 := v2.YggIP

		if !TestConnection(yggIP1, "22") {
			t.Errorf("yggdrasil IP 1 isn't reachable")
		}
		if !TestConnection(yggIP2, "22") {
			t.Errorf("yggdrasil IP 2 isn't reachable")
		}

		privateIP1 := v1.IP
		privateIP2 := v2.IP

		public1Ip6 := strings.Split(v1.ComputedIP6, "/")[0]
		public2Ip6 := strings.Split(v2.ComputedIP6, "/")[0]

		_, err = RemoteRun("root", yggIP1, "apt install -y netcat", privateKey)
		assert.NoError(t, err)

		_, err = RemoteRun("root", yggIP2, "apt install -y netcat", privateKey)
		assert.NoError(t, err)

		// check privateIP1 from vm2
		_, err = RemoteRun("root", yggIP1, "nc -z "+privateIP2+" 22", privateKey)
		assert.NoError(t, err)

		// check privateIP2 from vm1
		_, err = RemoteRun("root", yggIP2, "nc -z "+privateIP1+" 22", privateKey)
		assert.NoError(t, err)

		// check yggIP2 from vm1
		_, err = RemoteRun("root", yggIP1, "nc -z "+yggIP2+" 22", privateKey)
		assert.NoError(t, err)

		// check yggIP1 from vm2
		_, err = RemoteRun("root", yggIP2, "nc -z "+yggIP1+" 22", privateKey)
		assert.NoError(t, err)

		// check publicIP62 from vm1
		_, err = RemoteRun("root", yggIP1, "nc -z "+public2Ip6+" 22", privateKey)
		assert.NoError(t, err)

		// check publicIP61 from vm2
		_, err = RemoteRun("root", yggIP2, "nc -z "+public1Ip6+" 22", privateKey)
		assert.NoError(t, err)
	})

}
