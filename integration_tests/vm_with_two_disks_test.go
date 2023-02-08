// Package integration for integration tests
package integration

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestVMWithTwoDisk(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := NodeFilter{
		CRU:    2,
		SRU:    3,
		MRU:    1,
		Status: "up",
	}
	nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
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

	disk1 := workloads.Disk{
		Name:   "diskTest1",
		SizeGP: 1,
	}
	disk2 := workloads.Disk{
		Name:   "diskTest2",
		SizeGP: 2,
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		Mounts: []workloads.Mount{
			{DiskName: disk1.Name, MountPoint: "/disk1"},
			{DiskName: disk2.Name, MountPoint: "/disk2"},
		},
		IP:          "10.20.2.5",
		NetworkName: network.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("vm", nodeID, "", nil, network.Name, []workloads.Disk{disk1, disk2}, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.StateLoader.LoadVMFromGrid(nodeID, vm.Name)
	assert.NoError(t, err)

	resDisk1, err := tfPluginClient.StateLoader.LoadDiskFromGrid(nodeID, disk1.Name)
	assert.NoError(t, err)
	assert.Equal(t, disk1, resDisk1)

	resDisk2, err := tfPluginClient.StateLoader.LoadDiskFromGrid(nodeID, disk2.Name)
	assert.NoError(t, err)
	assert.Equal(t, disk2, resDisk2)

	yggIP := v.YggIP
	assert.NotEmpty(t, yggIP)
	if !TestConnection(yggIP, "22") {
		t.Errorf("yggdrasil ip is not reachable")
	}

	// Check that disk has been mounted successfully

	output, err := RemoteRun("root", yggIP, "df -h | grep -w /disk1", privateKey)
	assert.NoError(t, err)
	assert.Contains(t, string(output), fmt.Sprintf("%d.0G", disk1.SizeGP))

	output, err = RemoteRun("root", yggIP, "df -h | grep -w /disk2", privateKey)
	assert.NoError(t, err)
	assert.Contains(t, string(output), fmt.Sprintf("%d.0G", disk2.SizeGP))

	// create file -> d1, check file size, move file -> d2, check file size

	_, err = RemoteRun("root", yggIP, "dd if=/dev/vda bs=1M count=512 of=/disk1/test.txt", privateKey)
	assert.NoError(t, err)

	res, err := RemoteRun("root", yggIP, "du /disk1/test.txt | head -n1 | awk '{print $1;}' | tr -d -c 0-9", privateKey)
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(512*1024))

	_, err = RemoteRun("root", yggIP, "mv /disk1/test.txt /disk2/", privateKey)
	assert.NoError(t, err)

	res, err = RemoteRun("root", yggIP, "du /disk2/test.txt | head -n1 | awk '{print $1;}' | tr -d -c 0-9", privateKey)
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(512*1024))

	// create file -> d2, check file size, copy file -> d1, check file size

	_, err = RemoteRun("root", yggIP, "dd if=/dev/vdb bs=1M count=512 of=/disk2/test.txt", privateKey)
	assert.NoError(t, err)

	res, err = RemoteRun("root", yggIP, "du /disk2/test.txt | head -n1 | awk '{print $1;}' | tr -d -c 0-9", privateKey)
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(512*1024))

	_, err = RemoteRun("root", yggIP, "cp /disk2/test.txt /disk1/", privateKey)
	assert.NoError(t, err)

	res, err = RemoteRun("root", yggIP, "du /disk1/test.txt | head -n1 | awk '{print $1;}' | tr -d -c 0-9", privateKey)
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(512*1024))

	// copy same file -> d1 (not enough space)

	_, err = RemoteRun("root", yggIP, "cp /disk2/test.txt /disk1/test2.txt", privateKey)
	assert.Error(t, err)

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)
}
