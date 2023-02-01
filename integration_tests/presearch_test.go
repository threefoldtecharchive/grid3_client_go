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

func TestPresearchDeployment(t *testing.T) {
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

	network := workloads.ZNet{
		Name:        "presearchNetworkTest",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
	disk := workloads.Disk{
		Name: "diskTest",
		Size: 1,
	}

	vm := workloads.VM{
		Name:       "presearchTest",
		Flist:      "https://hub.grid.tf/tf-official-apps/presearch-v2.2.flist",
		CPU:        2,
		PublicIP:   true,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY":                     publicKey,
			"PRESEARCH_REGISTRATION_CODE": "e5083a8d0a6362c6cf7a3078bfac81e3",
		},
		Mounts: []workloads.Mount{
			{DiskName: disk.Name, MountPoint: "/var/lib/docker"},
		},
		NetworkName: network.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("presearch", nodeID, "", nil, network.Name, []workloads.Disk{disk}, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm.Name)
	assert.NoError(t, err)

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

	output, err := RemoteRun("root", yggIP, "cat /proc/1/environ", privateKey)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "PRESEARCH_REGISTRATION_CODE=e5083a8d0a6362c6cf7a3078bfac81e3")

	ticker := time.NewTicker(2 * time.Second)
	for now := time.Now(); time.Since(now) < 1*time.Minute; {
		<-ticker.C
		output, err = RemoteRun("root", yggIP, "zinit list", privateKey)
		if err == nil && strings.Contains(output, "prenode: Success") {
			break
		}
	}

	assert.NoError(t, err)
	assert.Contains(t, output, "prenode: Success")

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)

}
