package integration

import (
	"context"
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
		Node:      33,
		DiskSize:  1,
		PublicIP:  false,
		Planetary: true,
		Flist:     "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist",
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
		Nodes:       []uint32{33, 45},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	t.Run("master, 2 workers", func(t *testing.T) {

		err := cluster.Stage(context.Background(), manager)
		assert.NoError(t, err)

		_, err = network.Stage(context.Background(), apiClient)
		assert.NoError(t, err)

		err = manager.Commit(context.Background())
		assert.NoError(t, err)

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
		status := Wait(masterIP, "22")
		if status == false {
			t.Errorf("public ip not reachable")
		}

		time.Sleep(30 * (time.Second))
		res, errors := RemoteRun("root", masterIP, "kubectl get node")
		log.Printf("res: %s", res)
		assert.Empty(t, errors)

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

	t.Run("k8s with duplicate names", func(t *testing.T) {
		cluster.Workers[0].Name = cluster.Master.Name

		err := cluster.Stage(context.Background(), manager)
		assert.ErrorIs(t, err, workloads.ErrDuplicateName)
	})

}
