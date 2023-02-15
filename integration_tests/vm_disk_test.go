// Package integration for integration tests
package integration

import (
	"context"
	"fmt"

	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestVmDisk(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := deployer.NodeFilter{
		CRU:    2,
		SRU:    2,
		MRU:    1,
		Status: "up",
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

	disk := workloads.Disk{
		Name:   "diskTest",
		SizeGB: 1,
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		Mounts: []workloads.Mount{
			{DiskName: disk.Name, MountPoint: "/disk"},
		},
		IP:          "10.20.2.5",
		NetworkName: network.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("vm", nodeID, "", nil, network.Name, []workloads.Disk{disk}, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.State.LoadVMFromGrid(nodeID, vm.Name, dl.Name)
	assert.NoError(t, err)

	resDisk, err := tfPluginClient.State.LoadDiskFromGrid(nodeID, disk.Name, dl.Name)
	assert.NoError(t, err)
	assert.Equal(t, disk, resDisk)

	yggIP := v.YggIP
	assert.NotEmpty(t, yggIP)
	if !TestConnection(yggIP, "22") {
		t.Errorf("yggdrasil ip is not reachable")
	}

	// Check that disk has been mounted successfully
	output, err := RemoteRun("root", yggIP, "df -h | grep -w /disk", privateKey)
	assert.NoError(t, err)
	assert.Contains(t, string(output), fmt.Sprintf("%d.0G", disk.SizeGB))

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)
}
