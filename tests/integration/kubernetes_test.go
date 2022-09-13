package integration

import (
	"context"
	"fmt"

	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestKubernetes(t *testing.T) {
	manager, apiClient := setup()
	sshPublicKey := os.Getenv("PUBLICKEY")
	master := workloads.K8sNodeData{
		Name:      "ms",
		Node:      45,
		DiskSize:  1,
		PublicIP:  false,
		Planetary: true,
		Cpu:       1,
		Memory:    2048,
	}
	worker1 := master
	worker1.Name = "w1"
	worker1.Node = 45
	worker2 := worker1
	worker2.Name = "w2"

	cluster := workloads.K8sCluster{
		Master:      &master,
		Workers:     []workloads.K8sNodeData{worker1, worker2},
		Token:       "123456",
		SSHKey:      sshPublicKey,
		NetworkName: "skynet",
	}

	network := workloads.TargetNetwork{
		Name:        "skynet",
		Description: "not skynet",
		Nodes:       []uint32{45},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	network1 := workloads.TargetNetwork{
		Name:        "netVM",
		Description: "network for testing vm",
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
			"SSH_KEY":  sshPublicKey,
			"TEST_VAR": "this value for test",
		},
		IP:          "10.1.0.2",
		NetworkName: "netVM",
		PublicIP6:   true,
	}

	t.Run("cluster with 3 nodes", func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		_, err := network1.Stage(ctx, apiClient)
		assert.NoError(t, err)
		err = vm1.Stage(manager, 14)
		assert.NoError(t, err)
		err = manager.Commit(ctx)
		assert.NoError(t, err)
		defer manager.CancelAll()

		result, err := loader.LoadVmFromGrid(manager, 14, "vm1")
		assert.NoError(t, err)

		yggIP := result.YggIP
		privateIP := result.IP
		publicIP6 := strings.Split(result.ComputedIP6, "/")[0]

		fmt.Println(privateIP)
		fmt.Println(publicIP6)

		err = cluster.Stage(ctx, manager)
		assert.NoError(t, err)

		_, err = network.Stage(ctx, apiClient)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)
		defer cancelDeployments(t, manager)

		masterNode := map[uint32]string{master.Node: master.Name}
		workerNodes := map[uint32][]string{}
		for _, worker := range cluster.Workers {
			workerNodes[worker.Node] = append(workerNodes[worker.Node], worker.Name)
		}
		loadedCluster, err := loader.LoadK8sFromGrid(manager, masterNode, workerNodes)
		assert.NoError(t, err)

		log.Printf("loaded Cluster: %+v", loadedCluster)
		log.Printf("master: %+v", *(loadedCluster.Master))

		masterIP := loadedCluster.Master.YggIP

		if !Wait(masterIP, "22") {
			t.Errorf("yggdrasil IP for kuburnetes isn't reachable")
		}
		if !Wait(yggIP, "22") {
			t.Errorf("yggdrasil IP for vm isn't reachable")
		}

		_, err = RemoteRun("root", yggIP, "apt install -y netcat")
		assert.NoError(t, err)

		// check privateIP for kubernetes from vm
		_, err = RemoteRun("root", masterIP, "nc -z "+privateIP+" 22")
		assert.NoError(t, err)

		// check yggIP for from vm
		_, err = RemoteRun("root", yggIP, "nc -z "+masterIP+" 22")
		assert.NoError(t, err)

		time.Sleep(30 * (time.Second))
		res, err := RemoteRun("root", masterIP, "kubectl get node")
		log.Printf("res: %s", res)
		assert.NoError(t, err)

		res = strings.Trim(res, "\n")
		nodes := strings.Split(string(res), "\n")[1:]
		// assert that there are n+1 nodes (1 master and n workers)
		assert.Equal(t, 1+len(cluster.Workers), len(nodes))

		// Check that worker is ready
		for i := 0; i < len(nodes); i++ {
			assert.Contains(t, nodes[i], "Ready")
		}

		err = manager.CancelAll()
		assert.NoError(t, err)
	})

	t.Run("nodes with duplicate names", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cluster.Workers[0].Name = cluster.Master.Name

		err := cluster.Stage(ctx, manager)
		assert.ErrorIs(t, err, workloads.ErrDuplicateName)
	})

}
