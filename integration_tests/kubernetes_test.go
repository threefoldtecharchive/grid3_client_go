// Package integration for integration tests
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
	"github.com/threefoldtech/grid3-go/deployer"
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
		CPU:       1,
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

	network := workloads.ZNet{
		Name:        "skynet",
		Description: "not skynet",
		Nodes:       []uint32{45},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	networkManager, err := deployer.NewNetworkDeployer(apiClient.Manager, network)
	assert.NoError(t, err)

	t.Run("cluster with 3 nodes", func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		err := manager.Stage(&cluster, 0)
		assert.NoError(t, err)

		_, err = networkManager.Stage(ctx, apiClient, network)
		assert.NoError(t, err)

		err = manager.Commit(ctx)
		assert.NoError(t, err)

		err = manager.CancelAll()
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
	})

	t.Run("nodes with duplicate names", func(t *testing.T) {
		cluster.Workers[0].Name = cluster.Master.Name

		err := manager.Stage(&cluster, 0)
		assert.Error(t, err)
	})

}
