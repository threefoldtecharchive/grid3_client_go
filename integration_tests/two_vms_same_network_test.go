// Package integration for integration tests
package integration

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestTwoVMsSameNetwork(t *testing.T) {
	manager, apiClient := setup()
	publicKey := os.Getenv("PUBLICKEY")
	network := workloads.ZNet{
		Name:        "testingNetwork456",
		Description: "network for testing",
		Nodes:       []uint32{14},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
	vm1 := workloads.VM{
		Name:       "vm1",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-20.04.flist",
		CPU:        2,
		PublicIP6:  true,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		IP:          "10.1.0.2",
		NetworkName: "testingNetwork456",
	}
	vm2 := workloads.VM{
		Name:       "vm2",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-20.04.flist",
		CPU:        2,
		PublicIP6:  true,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		IP:          "10.1.0.3",
		NetworkName: "testingNetwork456",
	}

	networkManager, err := deployer.NewNetworkDeployer(apiClient.Manager, network)
	assert.NoError(t, err)

	t.Run("public ipv6 and yggdrasil", func(t *testing.T) {
		vm1Cp := vm1
		vm2Cp := vm2
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		_, err := networkManager.Stage(ctx, apiClient, network)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)

		err = manager.CancelAll()
		assert.NoError(t, err)

		err = manager.Stage(&vm1Cp, 14)
		assert.NoError(t, err)

		err = manager.Stage(&vm2Cp, 14)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)

		result1, err := loader.LoadVMFromGrid(manager, 14, "vm1")
		assert.NoError(t, err)

		result2, err := loader.LoadVMFromGrid(manager, 14, "vm2")
		assert.NoError(t, err)

		yggIP1 := result1.YggIP
		yggIP2 := result2.YggIP

		privateIP1 := result1.IP
		privateIP2 := result2.IP

		publicIP6_1 := strings.Split(result1.ComputedIP6, "/")[0]
		publicIP6_2 := strings.Split(result2.ComputedIP6, "/")[0]

		if !Wait(yggIP1, "22") {
			t.Errorf("Yggdrasil IP 1 not reachable")
		}
		if !Wait(yggIP2, "22") {
			t.Errorf("Yggdrasil IP 2 not reachable")
		}

		_, err = RemoteRun("root", yggIP1, "apt install -y netcat")
		assert.NoError(t, err)

		_, err = RemoteRun("root", yggIP2, "apt install -y netcat")
		assert.NoError(t, err)

		// check privateIP2 from vm1
		_, err = RemoteRun("root", yggIP2, "nc -z "+privateIP1+" 22")
		assert.NoError(t, err)

		// check privateIP1 from vm2
		_, err = RemoteRun("root", yggIP1, "nc -z "+privateIP2+" 22")
		assert.NoError(t, err)

		// check yggIP2 from vm1
		_, err = RemoteRun("root", yggIP1, "nc -z "+yggIP2+" 22")
		assert.NoError(t, err)

		// check yggIP1 from vm2
		_, err = RemoteRun("root", yggIP2, "nc -z "+yggIP1+" 22")
		assert.NoError(t, err)

		// check publicIP62 from vm1
		_, err = RemoteRun("root", yggIP1, "nc -z "+publicIP6_2+" 22")
		assert.NoError(t, err)

		// check publicIP61 from vm2
		_, err = RemoteRun("root", yggIP2, "nc -z "+publicIP6_1+" 22")
		assert.NoError(t, err)

	})
	t.Run("public IPv4", func(t *testing.T) {
		t.SkipNow()
		network.Nodes = []uint32{45}
		vm1Cp := vm1
		vm1Cp.PublicIP = true
		vm2Cp := vm2
		vm2Cp.PublicIP = true

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		_, err := networkManager.Stage(ctx, apiClient, network)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)

		err = manager.CancelAll()
		assert.NoError(t, err)

		err = manager.Stage(&vm1Cp, 45)
		assert.NoError(t, err)

		err = manager.Stage(&vm2Cp, 45)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)

		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		result1, err := loader.LoadVMFromGrid(manager, 45, "vm1")
		assert.NoError(t, err)

		result2, err := loader.LoadVMFromGrid(manager, 45, "vm2")
		assert.NoError(t, err)

		yggIP1 := result1.YggIP
		yggIP2 := result2.YggIP

		publicIP1 := result1.ComputedIP
		publicIP2 := result2.ComputedIP

		if !Wait(yggIP1, "22") {
			t.Errorf("Yggdrasil IP 1 not reachable")
		}
		if !Wait(yggIP2, "22") {
			t.Errorf("Yggdrasil IP 2 not reachable")
		}

		_, err = RemoteRun("root", yggIP1, "apt install -y netcat")
		assert.NoError(t, err)

		_, err = RemoteRun("root", yggIP2, "apt install -y netcat")
		assert.NoError(t, err)

		// check publicIP2 from vm1
		_, err = RemoteRun("root", yggIP1, "nc -z "+publicIP2+" 22")
		assert.NoError(t, err)

		// check publicIP1 from vm2
		_, err = RemoteRun("root", yggIP2, "nc -z "+publicIP1+" 22")
		assert.NoError(t, err)

	})
}
