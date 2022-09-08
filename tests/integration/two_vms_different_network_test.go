package integration

import (
	"context"
	"fmt"

	// "strings"

	"net"
	"os"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestTwoVmDifferentNet(t *testing.T) {
	manager, apiClient := setup()
	publicKey := os.Getenv("PUBLICKEY")
	network1 := workloads.TargetNetwork{
		Name:        "Network1",
		Description: "network for testing",
		Nodes:       []uint32{14},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}
	network2 := workloads.TargetNetwork{
		Name:        "Network2",
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
		Cpu:        2,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY":  publicKey,
			"TEST_VAR": "this value for test",
		},
		IP:          "10.1.0.2",
		NetworkName: "Network1",
		PublicIP6:   true,
	}
	vm2 := workloads.VM{
		Name:       "vm2",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-20.04.flist",
		Cpu:        2,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY":  publicKey,
			"TEST_VAR": "this value for test",
		},
		IP:          "10.1.0.3",
		NetworkName: "Network2",
		PublicIP6:   true,
	}

	// t.Run("check public ipv6 and yggdrasil", func(t *testing.T) {

	// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// 	defer cancel()

	// 	_, err := network1.Stage(ctx, apiClient)
	// 	assert.NoError(t, err)

	// 	_, err = network2.Stage(ctx, apiClient)
	// 	assert.NoError(t, err)

	// 	err = vm1.Stage(manager, 14)
	// 	assert.NoError(t, err)

	// 	err = vm2.Stage(manager, 14)
	// 	assert.NoError(t, err)

	// 	err = manager.Commit(ctx)
	// 	assert.NoError(t, err)
	// 	defer manager.CancelAll()

	// 	res1, err := loader.LoadVmFromGrid(manager, 14, "vm1")
	// 	assert.NoError(t, err)
	// 	res2, err := loader.LoadVmFromGrid(manager, 14, "vm2")
	// 	assert.NoError(t, err)
	// 	defer manager.CancelAll()

	// 	yggIP1 := res1.YggIP
	// 	yggIP2 := res2.YggIP

	// 	privateIP1 := res1.IP
	// 	privateIP2 := res2.IP

	// 	public1Ip6 := strings.Split(res1.ComputedIP6, "/")[0]
	// 	public2Ip6 := strings.Split(res2.ComputedIP6, "/")[0]

	// 	if !Wait(yggIP1, "22") {
	// 		t.Errorf("yggdrasil IP 1 isn't reachable")
	// 	}
	// 	if !Wait(yggIP2, "22") {
	// 		t.Errorf("yggdrasil IP 2 isn't reachable")
	// 	}

	// 	_, err = RemoteRun("root", yggIP1, "apt install -y netcat")
	// 	assert.NoError(t, err)

	// 	_, err = RemoteRun("root", yggIP2, "apt install -y netcat")
	// 	assert.NoError(t, err)

	// 	// check privateIP2 from vm1
	// 	_, err = RemoteRun("root", yggIP2, "nc -z "+privateIP1+" 22")
	// 	assert.NoError(t, err)

	// 	// check privateIP1 from vm2
	// 	_, err = RemoteRun("root", yggIP1, "nc -z "+privateIP2+" 22")
	// 	assert.NoError(t, err)

	// 	// check yggIP2 from vm1
	// 	_, err = RemoteRun("root", yggIP1, "nc -z "+yggIP2+" 22")
	// 	assert.NoError(t, err)

	// 	// check yggIP1 from vm2
	// 	_, err = RemoteRun("root", yggIP2, "nc -z "+yggIP1+" 22")
	// 	assert.NoError(t, err)

	// 	// check publicIP62 from vm1
	// 	_, err = RemoteRun("root", yggIP1, "nc -z "+public2Ip6+" 22")
	// 	assert.NoError(t, err)

	// 	// // check publicIP61 from vm2
	// 	_, err = RemoteRun("root", yggIP2, "nc -z "+public1Ip6+" 22")
	// 	assert.NoError(t, err)
	// })

	t.Run("check public ipv4", func(t *testing.T) {
		network1Tmp := network1
		network2Tmp := network2
		vm1Tmp := vm1
		vm2Tmp := vm2

		network1Tmp.Nodes = []uint32{45}
		network2Tmp.Nodes = []uint32{45}
		vm1Tmp.PublicIP = true
		vm2Tmp.PublicIP = true

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		_, err := network1Tmp.Stage(ctx, apiClient)
		assert.NoError(t, err)

		_, err = network2Tmp.Stage(ctx, apiClient)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)
		defer manager.CancelAll()

		err = vm1Tmp.Stage(manager, 45)
		assert.NoError(t, err)

		err = vm2Tmp.Stage(manager, 45)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)
		defer manager.CancelAll()

		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		res1, err := loader.LoadVmFromGrid(manager, 45, "vm1")
		assert.NoError(t, err)
		res2, err := loader.LoadVmFromGrid(manager, 45, "vm2")
		assert.NoError(t, err)
		defer manager.CancelAll()

		fmt.Print(res1)
		fmt.Println(res2)

	})

}
