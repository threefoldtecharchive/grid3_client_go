// Package integration for integration tests
package integration

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestVMDeployment(t *testing.T) {
	manager, apiClient := setup()
	publicKey := os.Getenv("PUBLICKEY")
	network := workloads.ZNet{
		Name:        "testingNetwork123",
		Description: "network for testing",
		Nodes:       []uint32{14},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
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
		IP:          "10.1.0.2",
		NetworkName: "testingNetwork123",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	networkManager, err := deployer.NewNetworkDeployer(apiClient.Manager, network)
	assert.NoError(t, err)

	t.Run("check VM configuration is correct", func(t *testing.T) {
		vmCp := vm

		_, err := networkManager.Stage(ctx, apiClient, network)
		assert.NoError(t, err)
		err = manager.Commit(ctx)
		assert.NoError(t, err)

		err = manager.CancelAll()
		assert.NoError(t, err)

		err = manager.Stage(&vmCp, 14)
		assert.NoError(t, err)
		err = manager.Commit(ctx)
		assert.NoError(t, err)

		result, err := loader.LoadVMFromGrid(manager, 14, "vm")
		assert.NoError(t, err)

		assert.Equal(t, 20*1024, result.RootfsSize)

		yggIP := result.YggIP
		assert.NotEmpty(t, yggIP)

		if !Wait(yggIP, "22") {
			t.Errorf("Yggdrasil IP not reachable")
		}

		res, err := RemoteRun("root", yggIP, "cat /proc/1/environ")
		assert.Contains(t, string(res), "TEST_VAR=this value for test")
		assert.NoError(t, err)

		res, err = RemoteRun("root", yggIP, "grep -c processor /proc/cpuinfo")
		assert.Equal(t, "2\n", res)
		assert.NoError(t, err)
		res, err = RemoteRun("root", yggIP, "grep MemTotal /proc/meminfo | tr -d -c 0-9")
		assert.NoError(t, err)
		resMem, err := strconv.Atoi(res)
		assert.NoError(t, err)
		assert.InDelta(t, resMem, 1024*1024, 0.1*1024*1024)

	})
	t.Run("check public ip is reachable", func(t *testing.T) {
		t.SkipNow()
		network.Nodes = []uint32{45}
		vmCp := vm
		vmCp.PublicIP = true

		_, err := networkManager.Stage(ctx, apiClient, network)
		assert.NoError(t, err)
		err = manager.Commit(ctx)
		assert.NoError(t, err)

		err = manager.CancelAll()
		assert.NoError(t, err)

		err = manager.Stage(&vmCp, 45)
		assert.NoError(t, err)
		err = manager.Commit(ctx)
		assert.NoError(t, err)

		result, err := loader.LoadVMFromGrid(manager, 45, "vm")
		assert.NoError(t, err)

		pIP := strings.Split(result.ComputedIP, "/")[0]
		if !Wait(pIP, "22") {
			t.Errorf("public IP not reachable")
		}

	})

}
