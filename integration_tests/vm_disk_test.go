// Package integration for integration tests
package integration

import (
	"context"
	"strconv"

	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/manager"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestVmDisk(t *testing.T) {
	dlManager, apiClient := setup()
	publicKey := os.Getenv("PUBLICKEY")
	network := workloads.ZNet{
		Name:        "networkalaa",
		Description: "network for testing",
		Nodes:       []uint32{14},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
	disk := workloads.Disk{
		Name: "testdisk",
		Size: 1,
	}
	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-20.04.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY":  publicKey,
			"TEST_VAR": "this value for test",
		},
		Mounts: []workloads.Mount{
			{DiskName: "testdisk", MountPoint: "/disk"},
		},
		IP:          "10.1.0.2",
		NetworkName: "networkalaa",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	networkManager, err := manager.NewNetworkDeployer(apiClient.Manager, network)
	assert.NoError(t, err)

	_, err = networkManager.Stage(ctx, apiClient, network)
	assert.NoError(t, err)

	err = dlManager.Stage(&vm, 14)
	assert.NoError(t, err)

	err = dlManager.Stage(&disk, 14)
	assert.NoError(t, err)

	err = dlManager.Commit(ctx)
	assert.NoError(t, err)

	err = dlManager.CancelAll()
	assert.NoError(t, err)

	result, err := manager.LoadVMFromGrid(dlManager, 14, "vm")
	assert.NoError(t, err)

	resDisk, err := manager.LoadDiskFromGrid(dlManager, 14, "testdisk")
	assert.NoError(t, err)
	assert.Equal(t, disk, resDisk)

	yggIP := result.YggIP

	res, err := RemoteRun("root", yggIP, "df /disk/ | tail -1 | awk '{print $2}' | tr -d '\\n'")
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(1024*1024))

	_, err = RemoteRun("root", yggIP, "dd if=/dev/vda bs=1M count=512 of=/disk/test.txt")
	assert.NoError(t, err)

	res, err = RemoteRun("root", yggIP, "du /disk/test.txt | head -n1 | awk '{print $1;}' | tr -d -c 0-9")
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(512*1024))

	_, err = RemoteRun("root", yggIP, "rm /disk/test.txt")
	assert.NoError(t, err)

	res, err = RemoteRun("root", yggIP, "df /disk/ | tail -1 | awk '{print $2}' | tr -d '\\n'")
	assert.NoError(t, err)
	assert.Equal(t, res, strconv.Itoa(1024*1024))

}
