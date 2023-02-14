// Package integration for integration tests
package integration

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const (
	DataZDBNum = 4
	MetaZDBNum = 4
)

func TestQSFSDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := deployer.NodeFilter{
		Status: "up",
		SRU:    10,
	}
	nodeIDs, err := deployer.FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
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

	dataZDBs := []workloads.ZDB{}
	metaZDBs := []workloads.ZDB{}
	for i := 1; i <= DataZDBNum; i++ {
		zdb := workloads.ZDB{
			Name:        "qsfsDataZdb" + strconv.Itoa(i),
			Password:    "password",
			Public:      true,
			Size:        1,
			Description: "zdb for testing",
			Mode:        zos.ZDBModeSeq,
		}
		dataZDBs = append(dataZDBs, zdb)
	}

	for i := 1; i <= MetaZDBNum; i++ {
		zdb := workloads.ZDB{
			Name:        "qsfsMetaZdb" + strconv.Itoa(i),
			Password:    "password",
			Public:      true,
			Size:        1,
			Description: "zdb for testing",
			Mode:        zos.ZDBModeUser,
		}
		metaZDBs = append(metaZDBs, zdb)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	dl := workloads.NewDeployment("qsfs", nodeID, "", nil, "", nil, append(dataZDBs, metaZDBs...), nil, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	// result zdbs
	resDataZDBs := []workloads.ZDB{}
	resMetaZDBs := []workloads.ZDB{}
	for i := 1; i <= DataZDBNum; i++ {
		res, err := tfPluginClient.State.LoadZdbFromGrid(nodeID, "qsfsDataZdb"+strconv.Itoa(i))
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
		resDataZDBs = append(resDataZDBs, res)
	}
	for i := 1; i <= MetaZDBNum; i++ {
		res, err := tfPluginClient.State.LoadZdbFromGrid(nodeID, "qsfsMetaZdb"+strconv.Itoa(i))
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
		resMetaZDBs = append(resMetaZDBs, res)
	}

	// backends
	dataBackends := []workloads.Backend{}
	metaBackends := []workloads.Backend{}
	for i := 0; i < DataZDBNum; i++ {
		dataBackends = append(dataBackends, workloads.Backend{
			Address:   "[" + resDataZDBs[i].IPs[1] + "]" + ":" + fmt.Sprint(resDataZDBs[i].Port),
			Namespace: resDataZDBs[i].Namespace,
			Password:  resDataZDBs[i].Password})
	}
	for i := 0; i < MetaZDBNum; i++ {
		metaBackends = append(metaBackends, workloads.Backend{
			Address:   "[" + resMetaZDBs[i].IPs[1] + "]" + ":" + fmt.Sprint(resMetaZDBs[i].Port),
			Namespace: resMetaZDBs[i].Namespace,
			Password:  resMetaZDBs[i].Password})
	}

	qsfs := workloads.QSFS{
		Name:                 "qsfsTest",
		Description:          "qsfs for testing",
		Cache:                1024,
		MinimalShards:        2,
		ExpectedShards:       4,
		RedundantGroups:      0,
		RedundantNodes:       0,
		MaxZDBDataDirSize:    512,
		EncryptionAlgorithm:  "AES",
		EncryptionKey:        "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
		CompressionAlgorithm: "snappy",
		Groups:               workloads.Groups{{Backends: dataBackends}},
		Metadata: workloads.Metadata{
			Type:                "zdb",
			Prefix:              "test",
			EncryptionAlgorithm: "AES",
			EncryptionKey:       "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
			Backends:            metaBackends,
		},
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		Mounts: []workloads.Mount{
			{DiskName: qsfs.Name, MountPoint: "/qsfs"},
		},
		NetworkName: network.Name,
	}

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl = workloads.NewDeployment("qsfs", nodeID, "", nil, network.Name, nil, append(dataZDBs, metaZDBs...), []workloads.VM{vm}, []workloads.QSFS{qsfs})
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	resVM, err := tfPluginClient.State.LoadVMFromGrid(nodeID, vm.Name)
	assert.NoError(t, err)

	resQSFS, err := tfPluginClient.State.LoadQSFSFromGrid(nodeID, qsfs.Name)
	assert.NoError(t, err)
	assert.NotEmpty(t, resQSFS.MetricsEndpoint)

	// Check that the outputs not empty
	metrics := resQSFS.MetricsEndpoint
	assert.NotEmpty(t, metrics)

	yggIP := resVM.YggIP
	assert.NotEmpty(t, yggIP)
	if !TestConnection(yggIP, "22") {
		t.Errorf("yggdrasil ip is not reachable")
	}

	// get metrics
	cmd := exec.Command("curl", metrics)
	output, err := cmd.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(output), "fs_syscalls{syscall=\"create\"} 0")

	// try write to a file in mounted disk
	_, err = RemoteRun("root", yggIP, "cd /qsfs && echo hamadatext >> hamadafile", privateKey)
	assert.NoError(t, err)

	// get metrics after write
	cmd = exec.Command("curl", metrics)
	output, err = cmd.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(output), "fs_syscalls{syscall=\"create\"} 1")

	resQSFS.MetricsEndpoint = ""
	assert.Equal(t, qsfs, resQSFS)

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)

	_, err = tfPluginClient.State.LoadQSFSFromGrid(nodeID, qsfs.Name)
	assert.Error(t, err)
}
