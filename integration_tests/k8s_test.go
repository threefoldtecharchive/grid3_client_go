package integration

import (
	"context"

	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func AssertNodesAreReady(t *testing.T, workload *workloads.K8sCluster, privateKey string) {
	t.Helper()

	masterYggIP := workload.Master.YggIP
	assert.NotEmpty(t, masterYggIP)

	time.Sleep(5 * time.Second)
	output, err := RemoteRun("root", masterYggIP, "kubectl get node", privateKey)
	output = strings.TrimSpace(output)
	assert.Empty(t, err)

	nodesNumber := reflect.ValueOf(workload.Workers).Len() + 1
	numberOfReadynodes := strings.Count(output, "Ready")
	assert.True(t, numberOfReadynodes == nodesNumber, "number of ready nodes is not equal to number of nodes only %s nodes are ready", numberOfReadynodes)

}

func TestK8sDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := NodeFilter{
		CRU:    2,
		SRU:    2,
		MRU:    4,
		Status: "up",
	}
	nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	k8sNodeID := nodeIDs[0]

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{k8sNodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	flist := "https://hub.grid.tf/tf-official-apps/base:latest.flist"
	flistCheckSum, err := workloads.GetFlistChecksum(flist)
	assert.NoError(t, err)

	master := workloads.K8sNodeData{
		Name:          "K8sforTesting",
		Node:          k8sNodeID,
		DiskSize:      5,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     true,
		Flist:         "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workerNodeData1 := workloads.K8sNodeData{
		Name:          "worker1",
		Node:          k8sNodeID,
		DiskSize:      5,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     false,
		Flist:         "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workerNodeData2 := workloads.K8sNodeData{
		Name:          "worker2",
		Node:          k8sNodeID,
		DiskSize:      5,
		PublicIP:      false,
		PublicIP6:     false,
		Planetary:     false,
		Flist:         "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}

	workers := [2]workloads.K8sNodeData{workerNodeData1, workerNodeData2}

	k8sCluster := workloads.K8sCluster{
		Master:           &master,
		Workers:          workers[:],
		Token:            "token",
		SSHKey:           publicKey,
		NetworkName:      "testingNetwork",
		NodesIPRange:     make(map[uint32]gridtypes.IPNet),
		NodeDeploymentID: map[uint32]uint64{},
		ContractID:       0,
	}

	err = tfPluginClient.K8sDeployer.Deploy(ctx, &k8sCluster)
	assert.NoError(t, err)

	masterMap := map[uint32]string{master.Node: master.Name}
	workerMap := map[uint32][]string{workerNodeData1.Node: []string{workerNodeData1.Name, workerNodeData2.Name}}

	result, err := tfPluginClient.StateLoader.LoadK8sFromGrid(masterMap, workerMap)
	assert.NoError(t, err)
	assert.Equal(t, k8sCluster, result)

	// Check that the outputs not empty
	masterIP := result.Master.YggIP
	assert.NotEmpty(t, masterIP)

	// // Check wireguard config in output
	// // wgConfig :=
	// // assert.NotEmpty(t, wgConfig)

	// // Check that master is reachable
	// // testing connection on port 22, waits at max 3mins until it becomes ready otherwise it fails
	ok := TestConnection(masterIP, "22")
	assert.True(t, ok)

	// // // ssh to master node
	AssertNodesAreReady(t, &result, privateKey)

}
