// Package deployer is the grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

var twinID uint32 = 13
var contractID uint64 = 100
var nodeID uint32 = 10

func constructTestDeployment() workloads.Deployment {
	disks := []workloads.Disk{
		{
			Name:        "disk1",
			Size:        1024,
			Description: "disk1_description",
		},
		{
			Name:        "disk2",
			Size:        2048,
			Description: "disk2_description",
		},
	}

	zdbs := []workloads.ZDB{
		{
			Name:        "zdb1",
			Password:    "pass1",
			Public:      true,
			Size:        1024,
			Description: "zdb_description",
			Mode:        "data",
			IPs: []string{
				"::1",
				"::2",
			},
			Port:      9000,
			Namespace: "ns1",
		},
		{
			Name:        "zdb2",
			Password:    "pass2",
			Public:      true,
			Size:        1024,
			Description: "zdb2_description",
			Mode:        "meta",
			IPs: []string{
				"::3",
				"::4",
			},
			Port:      9001,
			Namespace: "ns2",
		},
	}

	vms := []workloads.VM{
		{
			Name:          "vm1",
			Flist:         "https://hub.grid.tf/tf-official-apps/discourse-v4.0.flist",
			FlistChecksum: "",
			PublicIP:      true,
			PublicIP6:     true,
			Planetary:     true,
			Corex:         true,
			ComputedIP:    "5.5.5.5/24",
			ComputedIP6:   "::7/64",
			YggIP:         "::8/64",
			IP:            "10.1.0.2",
			Description:   "vm1_description",
			CPU:           1,
			Memory:        1024,
			RootfsSize:    1024,
			Entrypoint:    "/sbin/zinit init",
			Mounts: []workloads.Mount{
				{
					DiskName:   "disk1",
					MountPoint: "/data1",
				},
				{
					DiskName:   "disk2",
					MountPoint: "/data2",
				},
			},
			Zlogs: []workloads.Zlog{
				{
					Zmachine: "vm1",
					Output:   "redis://codescalers1.com",
				},
				{
					Zmachine: "vm1",
					Output:   "redis://threefold1.io",
				},
			},
			EnvVars: map[string]string{
				"ssh_key":  "asd",
				"ssh_key2": "asd2",
			},
			NetworkName: "network",
		},
		{
			Name:          "vm2",
			Flist:         "https://hub.grid.tf/omar0.3bot/omarelawady-ubuntu-20.04.flist",
			FlistChecksum: "f0ae02b6244db3a5f842decd082c4e08",
			PublicIP:      false,
			PublicIP6:     true,
			Planetary:     true,
			Corex:         true,
			ComputedIP:    "",
			ComputedIP6:   "::7/64",
			YggIP:         "::8/64",
			IP:            "10.1.0.2",
			Description:   "vm2_description",
			CPU:           1,
			Memory:        1024,
			RootfsSize:    1024,
			Entrypoint:    "/sbin/zinit init",
			Mounts: []workloads.Mount{
				{
					DiskName:   "disk1",
					MountPoint: "/data1",
				},
				{
					DiskName:   "disk2",
					MountPoint: "/data2",
				},
			},
			Zlogs: []workloads.Zlog{
				{
					Zmachine: "vm2",
					Output:   "redis://codescalers.com",
				},
				{
					Zmachine: "vm2",
					Output:   "redis://threefold.io",
				},
			},
			EnvVars: map[string]string{
				"ssh_key":  "asd",
				"ssh_key2": "asd2",
			},
			NetworkName: "network",
		},
	}

	qsfs := []workloads.QSFS{
		{
			Name:                 "name1",
			Description:          "description1",
			Cache:                1024,
			MinimalShards:        4,
			ExpectedShards:       4,
			RedundantGroups:      0,
			RedundantNodes:       0,
			MaxZDBDataDirSize:    512,
			EncryptionAlgorithm:  "AES",
			EncryptionKey:        "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
			CompressionAlgorithm: "snappy",
			Metadata: workloads.Metadata{
				Type:                "zdb",
				Prefix:              "hamada",
				EncryptionAlgorithm: "AES",
				EncryptionKey:       "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
				Backends: workloads.Backends{
					{
						Address:   "[::10]:8080",
						Namespace: "ns1",
						Password:  "123",
					},
					{
						Address:   "[::11]:8080",
						Namespace: "ns2",
						Password:  "1234",
					},
					{
						Address:   "[::12]:8080",
						Namespace: "ns3",
						Password:  "1235",
					},
					{
						Address:   "[::13]:8080",
						Namespace: "ns4",
						Password:  "1236",
					},
				},
			},
			Groups: workloads.Groups{
				{
					Backends: workloads.Backends{
						{
							Address:   "[::110]:8080",
							Namespace: "ns5",
							Password:  "123",
						},
						{
							Address:   "[::111]:8080",
							Namespace: "ns6",
							Password:  "1234",
						},
						{
							Address:   "[::112]:8080",
							Namespace: "ns7",
							Password:  "1235",
						},
						{
							Address:   "[::113]:8080",
							Namespace: "ns8",
							Password:  "1236",
						},
					},
				},
			},
			MetricsEndpoint: "http://[::12]:9090/metrics",
		},
	}

	return workloads.Deployment{
		ContractID:  contractID,
		NodeID:      10,
		Disks:       disks,
		Zdbs:        zdbs,
		Vms:         vms,
		Qsfs:        qsfs,
		NetworkName: "network",
	}
}

func constructTestDeployer(t *testing.T, mock bool) (DeploymentDeployer, *mocks.RMBMockClient, *mocks.MockSubstrateExt, *mocks.MockNodeClientGetter, *mocks.MockDeployer) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	deployer := mocks.NewMockDeployer(ctrl)

	if mock {
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl

		tfPluginClient.StateLoader.ncPool = ncPool
		tfPluginClient.StateLoader.substrate = sub

		tfPluginClient.DeploymentDeployer.tfPluginClient = &tfPluginClient
		tfPluginClient.DeploymentDeployer.deployer = deployer
	}

	return tfPluginClient.DeploymentDeployer, cl, sub, ncPool, deployer
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	r, err := json.Marshal(v)
	assert.NoError(t, err)
	return r
}

func musUnmarshal(t *testing.T, bs json.RawMessage, v interface{}) {
	err := json.Unmarshal(bs, v)
	assert.NoError(t, err)
}

func TestDeploymentValidate(t *testing.T) {
	dl := constructTestDeployment()
	d, _, _, _, _ := constructTestDeployer(t, false)

	network := dl.NetworkName
	checksum := dl.Vms[0].FlistChecksum
	dl.NetworkName = network

	dl.Vms[0].FlistChecksum += " "
	assert.Error(t, d.Validate(context.Background(), &dl))

	dl.Vms[0].FlistChecksum = checksum
	assert.NoError(t, d.Validate(context.Background(), &dl))
}

func TestDeploymentSyncDeletedContract(t *testing.T) {
	dl := constructTestDeployment()
	d, _, sub, _, deployer := constructTestDeployer(t, true)

	sub.EXPECT().IsValidContract(dl.ContractID).Return(false, nil).AnyTimes()

	err := d.syncContract(context.Background(), &dl)
	assert.NoError(t, err)
	assert.Equal(t, dl.ContractID, uint64(0))
	dl.ContractID = contractID

	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{dl.NodeID: dl.ContractID}
	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{nodeID: contractID}).
		Return(map[uint32]gridtypes.Deployment{}, nil)

	assert.NoError(t, d.Sync(context.Background(), &dl))

	assert.Equal(t, dl.ContractID, uint64(0))
	assert.Empty(t, dl.Vms)
	assert.Empty(t, dl.Disks)
	assert.Empty(t, dl.Qsfs)
	assert.Empty(t, dl.Zdbs)
}

func TestDeploymentGenerateDeployment(t *testing.T) {
	dl := constructTestDeployment()
	d, cl, sub, ncPool, _ := constructTestDeployer(t, true)

	gridDl, err := dl.ZosDeployment(twinID)
	assert.NoError(t, err)

	net := constructTestNetwork()
	workload := net.ZosWorkload(net.NodesIPRange[nodeID], "", uint16(0), []zos.Peer{})
	networkDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{workload})

	d.tfPluginClient.StateLoader.currentNodeNetwork[nodeID] = contractID
	d.tfPluginClient.StateLoader.networks = networkState{net.Name: network{subnets: map[uint32]string{nodeID: net.IPRange.String()}}}

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(nodeID)).
		Return(client.NewNodeClient(twinID, cl), nil)

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = networkDl
			return nil
		}).AnyTimes()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &dl)
	assert.NoError(t, err)

	assert.Equal(t, len(gridDl.Workloads), len(dls[dl.NodeID].Workloads))
	assert.Equal(t, gridDl.Workloads, dls[dl.NodeID].Workloads)
}

func TestDeploymentSync(t *testing.T) {
	dl := constructTestDeployment()
	d, cl, sub, ncPool, deployer := constructTestDeployer(t, true)

	net := constructTestNetwork()
	workload := net.ZosWorkload(net.NodesIPRange[nodeID], "", uint16(0), []zos.Peer{})
	networkDl := workloads.NewGridDeployment(twinID, []gridtypes.Workload{workload})

	d.tfPluginClient.StateLoader.currentNodeNetwork[nodeID] = contractID
	d.tfPluginClient.StateLoader.networks = networkState{
		net.Name: network{
			subnets: map[uint32]string{
				nodeID: net.IPRange.String(),
			},
			nodeDeploymentHostIDs: make(nodeDeploymentHostIDs),
		},
	}

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(nodeID)).
		Return(client.NewNodeClient(twinID, cl), nil)

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = networkDl
			return nil
		}).AnyTimes()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &dl)
	assert.NoError(t, err)

	gridDl := dls[dl.NodeID]
	err = json.NewEncoder(log.Writer()).Encode(gridDl.Workloads)
	assert.NoError(t, err)

	for _, zlog := range gridDl.ByType(zos.ZLogsType) {
		*zlog.Workload = zlog.WithResults(gridtypes.Result{
			State: gridtypes.StateOk,
		})
	}

	for _, disk := range gridDl.ByType(zos.ZMountType) {
		*disk.Workload = disk.WithResults(gridtypes.Result{
			State: gridtypes.StateOk,
		})
	}

	wl, err := gridDl.Get(gridtypes.Name(dl.Vms[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZMachineResult{
			IP:    dl.Vms[0].IP,
			YggIP: dl.Vms[0].YggIP,
		}),
	})

	dataI, err := wl.WorkloadData()
	assert.NoError(t, err)

	data := dataI.(*zos.ZMachine)
	pubIP, err := gridDl.Get(data.Network.PublicIP)
	assert.NoError(t, err)

	*pubIP.Workload = pubIP.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.PublicIPResult{
			IP:   gridtypes.MustParseIPNet(dl.Vms[0].ComputedIP),
			IPv6: gridtypes.MustParseIPNet(dl.Vms[0].ComputedIP6),
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(dl.Vms[1].Name))
	assert.NoError(t, err)

	dataI, err = wl.WorkloadData()
	assert.NoError(t, err)

	data = dataI.(*zos.ZMachine)
	pubIP, err = gridDl.Get(data.Network.PublicIP)
	assert.NoError(t, err)

	*pubIP.Workload = pubIP.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.PublicIPResult{
			IPv6: gridtypes.MustParseIPNet(dl.Vms[1].ComputedIP6),
		}),
	})

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZMachineResult{
			IP:    dl.Vms[1].IP,
			YggIP: dl.Vms[1].YggIP,
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(dl.Qsfs[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.QuatumSafeFSResult{
			MetricsEndpoint: dl.Qsfs[0].MetricsEndpoint,
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(dl.Zdbs[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZDBResult{
			Namespace: dl.Zdbs[0].Namespace,
			IPs:       dl.Zdbs[0].IPs,
			Port:      uint(dl.Zdbs[0].Port),
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(dl.Zdbs[1].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZDBResult{
			Namespace: dl.Zdbs[1].Namespace,
			IPs:       dl.Zdbs[1].IPs,
			Port:      uint(dl.Zdbs[1].Port),
		}),
	})

	for i := 0; 2*i < len(gridDl.Workloads); i++ {
		gridDl.Workloads[i], gridDl.Workloads[len(gridDl.Workloads)-1-i] =
			gridDl.Workloads[len(gridDl.Workloads)-1-i], gridDl.Workloads[i]
	}

	sub.EXPECT().IsValidContract(contractID).Return(true, nil)

	var cp workloads.Deployment
	musUnmarshal(t, mustMarshal(t, dl), &cp)

	_, err = workloads.GetUsedIPs(gridDl)
	assert.NoError(t, err)

	deployer.EXPECT().
		GetDeployments(gomock.Any(), map[uint32]uint64{}).
		Return(map[uint32]gridtypes.Deployment{nodeID: gridDl}, nil)
	//manager.EXPECT().Commit(context.Background()).AnyTimes()
	assert.NoError(t, d.Sync(context.Background(), &dl))
	assert.Equal(t, dl.Vms, cp.Vms)
	assert.Equal(t, dl.Disks, cp.Disks)
	assert.Equal(t, dl.Qsfs, cp.Qsfs)
	assert.Equal(t, dl.Zdbs, cp.Zdbs)
	assert.Equal(t, dl.ContractID, cp.ContractID)
	assert.Equal(t, dl.NodeID, cp.NodeID)
}

func TestDeploymentDeploy(t *testing.T) {
	dl := constructTestDeployment()
	d, _, sub, _, deployer := constructTestDeployer(t, true)
	dl.SolutionType = "Virtual Machine"

	net := constructTestNetwork()

	dl.ContractID = 0
	d.tfPluginClient.StateLoader.currentNodeNetwork[nodeID] = contractID
	d.tfPluginClient.StateLoader.networks = networkState{
		net.Name: network{
			subnets: map[uint32]string{
				nodeID: net.IPRange.String(),
			},
			nodeDeploymentHostIDs: make(nodeDeploymentHostIDs),
		},
	}
	dls, err := d.GenerateVersionlessDeployments(context.Background(), &dl)
	assert.NoError(t, err)
	newDeploymentsSolutionProvider := map[uint32]*uint64{nodeID: nil}
	deploymentData := workloads.DeploymentData{
		Name:        dl.Name,
		Type:        "vm",
		ProjectName: "Virtual Machine",
	}
	newDeploymentsData := map[uint32]workloads.DeploymentData{nodeID: deploymentData}
	t.Run("Validation failed", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(10),
				},
			}, nil)

		assert.Error(t, d.Deploy(context.Background(), &dl))

		// nothing should change
		assert.Empty(t, dl.NodeDeploymentID)
		assert.Empty(t, dl.ContractID)
		assert.Empty(t, d.tfPluginClient.StateLoader.currentNodeDeployment)
	})
	t.Run("Deploying failed", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(100000),
				},
			}, nil)

		deployer.EXPECT().Deploy(
			gomock.Any(),
			map[uint32]uint64{},
			dls,
			newDeploymentsData,
			newDeploymentsSolutionProvider,
		).Return(map[uint32]uint64{}, errors.New("error"))

		assert.Error(t, d.Deploy(context.Background(), &dl))

		// nothing should change
		assert.Empty(t, dl.NodeDeploymentID)
		assert.Empty(t, dl.ContractID)
		assert.Empty(t, d.tfPluginClient.StateLoader.currentNodeDeployment)
	})
	t.Run("Deploying succeeded", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(100000),
				},
			}, nil)

		deployer.EXPECT().Deploy(
			gomock.Any(),
			map[uint32]uint64{},
			dls,
			newDeploymentsData,
			newDeploymentsSolutionProvider,
		).Return(map[uint32]uint64{nodeID: contractID}, nil)
		assert.NoError(t, d.Deploy(context.Background(), &dl))

		// should reflect on deployment and state
		assert.Equal(t, dl.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, dl.ContractID, contractID)
		assert.Equal(t, d.tfPluginClient.StateLoader.currentNodeDeployment, map[uint32]uint64{nodeID: contractID})
	})

}

func TestDeploymentCancel(t *testing.T) {
	dl := constructTestDeployment()
	d, _, sub, _, deployer := constructTestDeployer(t, true)
	dl.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

	d.tfPluginClient.StateLoader.currentNodeDeployment = map[uint32]uint64{nodeID: contractID}
	t.Run("Validation failed", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(10),
				},
			}, nil)

		assert.Error(t, d.Cancel(context.Background(), &dl))

		// nothing should change
		assert.Equal(t, dl.ContractID, contractID)
		assert.Equal(t, d.tfPluginClient.StateLoader.currentNodeDeployment, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, dl.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})

	})
	t.Run("Canceling failed", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(100000),
				},
			}, nil)

		deployer.EXPECT().
			Cancel(gomock.Any(), map[uint32]uint64{nodeID: contractID}, map[uint32]gridtypes.Deployment{}).
			Return(map[uint32]uint64{nodeID: contractID}, errors.New("error"))

		assert.Error(t, d.Cancel(context.Background(), &dl))

		// nothing should change
		assert.Equal(t, dl.ContractID, contractID)
		assert.Equal(t, d.tfPluginClient.StateLoader.currentNodeDeployment, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, dl.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})

	})
	t.Run("Canceling succeeded", func(t *testing.T) {
		sub.EXPECT().
			GetBalance(d.tfPluginClient.Identity).
			Return(substrate.Balance{
				Free: types.U128{
					Int: big.NewInt(100000),
				},
			}, nil)

		deployer.EXPECT().
			Cancel(gomock.Any(), map[uint32]uint64{nodeID: contractID}, map[uint32]gridtypes.Deployment{}).
			Return(map[uint32]uint64{}, nil)
		assert.NoError(t, d.Cancel(context.Background(), &dl))

		// should reflect on state and deployment
		assert.Empty(t, dl.ContractID)
		assert.Empty(t, d.tfPluginClient.StateLoader.currentNodeDeployment[dl.NodeID])
		assert.Empty(t, dl.NodeDeploymentID)

	})
}
