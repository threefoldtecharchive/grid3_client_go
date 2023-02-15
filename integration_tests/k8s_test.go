package integration

import (
	"context"
	"reflect"
	"strings"

	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func AssertNodesAreReady(t *testing.T, k8sCluster *workloads.K8sCluster, privateKey string) {
	t.Helper()

	masterYggIP := k8sCluster.Master.YggIP
	assert.NotEmpty(t, masterYggIP)

	// Check that the outputs not empty
	time.Sleep(5 * time.Second)
	output, err := RemoteRun("root", masterYggIP, "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && kubectl get node", privateKey)
	output = strings.TrimSpace(output)
	assert.Empty(t, err)

	nodesNumber := reflect.ValueOf(k8sCluster.Workers).Len() + 1
	numberOfReadyNodes := strings.Count(output, "Ready")
	assert.True(t, numberOfReadyNodes == nodesNumber, "number of ready nodes is not equal to number of nodes only %s nodes are ready", numberOfReadyNodes)
}

func TestK8sDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := deployer.NodeFilter{
		CRU:    2,
		SRU:    4,
		MRU:    4,
		Status: "up",
	}
	nodeIDs, err := deployer.FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	masterNodeID := nodeIDs[0]
	workerNodeID := nodeIDs[1]

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{masterNodeID, workerNodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	flist := "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist"
	flistCheckSum, err := workloads.GetFlistChecksum(flist)
	assert.NoError(t, err)

	master := workloads.K8sNode{
		Name:          "K8sForTesting",
		Node:          masterNodeID,
		DiskSize:      1,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     true,
		Flist:         "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workerNodeData1 := workloads.K8sNode{
		Name:          "worker1",
		Node:          workerNodeID,
		DiskSize:      1,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     false,
		Flist:         "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workerNodeData2 := workloads.K8sNode{
		Name:          "worker2",
		Node:          workerNodeID,
		DiskSize:      1,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     false,
		Flist:         "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workers := [2]workloads.K8sNode{workerNodeData1, workerNodeData2}

	k8sCluster := workloads.K8sCluster{
		Master:      &master,
		Workers:     workers[:],
		Token:       "tokens",
		SSHKey:      publicKey,
		NetworkName: "testingNetwork",
	}

	err = tfPluginClient.K8sDeployer.Deploy(ctx, &k8sCluster)
	assert.NoError(t, err)

	masterMap := map[uint32]string{master.Node: master.Name}
	workerMap := map[uint32][]string{workerNodeData1.Node: {workerNodeData1.Name, workerNodeData2.Name}}

	result, err := tfPluginClient.State.LoadK8sFromGrid(masterMap, workerMap, k8sCluster.Master.Name)
	assert.NoError(t, err)

	// check workers count
	assert.Equal(t, len(result.Workers), 2)

	// Check that master is reachable
	masterIP := result.Master.YggIP
	assert.NotEmpty(t, masterIP)

	// Check wireguard config in output
	wgConfig := network.AccessWGConfig
	assert.NotEmpty(t, wgConfig)

	// testing connection on port 22, waits at max 3mins until it becomes ready otherwise it fails
	ok := TestConnection(masterIP, "22")
	assert.True(t, ok)

	// ssh to master node
	AssertNodesAreReady(t, &result, privateKey)

	// cancel deployments
	err = tfPluginClient.K8sDeployer.Cancel(ctx, &k8sCluster)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)

	_, err = tfPluginClient.State.LoadK8sFromGrid(masterMap, workerMap, k8sCluster.Master.Name)
	assert.Error(t, err)
}
