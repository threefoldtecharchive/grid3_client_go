// Package deployer is the grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"log"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
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
			IP:            "10.10.10.10",
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
					Output: "redis://codescalers1.com",
				},
				{
					Output: "redis://threefold1.io",
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
			IP:            "10.10.10.10",
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
					Output: "redis://codescalers.com",
				},
				{
					Output: "redis://threefold.io",
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

func constructTestDeployer(t *testing.T, mock bool) (DeploymentDeployer, *mocks.RMBMockClient, *mocks.MockSubstrateExt, *mocks.MockNodeClientGetter) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	if mock {
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl

		tfPluginClient.stateLoader.ncPool = ncPool
		tfPluginClient.stateLoader.substrate = sub
	}

	return NewDeploymentDeployer(&tfPluginClient), cl, sub, ncPool
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

func TestValidate(t *testing.T) {
	dl := constructTestDeployment()
	d, _, _, _ := constructTestDeployer(t, false)

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
	d, _, sub, _ := constructTestDeployer(t, false)

	sub.EXPECT().IsValidContract(dl.ContractID).Return(false, nil).AnyTimes()

	contractID, err := d.syncContract(dl.ContractID)
	assert.NoError(t, err)
	assert.Equal(t, contractID, uint64(0))

	assert.NoError(t, d.Sync(context.Background()))

	dl = d.currentDeployments[dl.ContractID]
	assert.Equal(t, dl.ContractID, uint64(0))
	assert.Empty(t, dl.Vms)
	assert.Empty(t, dl.Disks)
	assert.Empty(t, dl.Qsfs)
	assert.Empty(t, dl.Zdbs)
}

func TestDeploymentGenerateDeployment(t *testing.T) {
	dl := constructTestDeployment()
	d, cl, sub, ncPool := constructTestDeployer(t, true)

	gridDl, err := dl.ConstructGridDeployment(twinID)
	assert.NoError(t, err)

	_, networkDl := constructTestNetwork()
	d.TFPluginClient.stateLoader.currentNodeNetwork[nodeID] = contractID

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
	d, cl, sub, ncPool := constructTestDeployer(t, true)

	_, networkDl := constructTestNetwork()
	d.TFPluginClient.stateLoader.currentNodeNetwork[nodeID] = contractID

	// invalidate contract
	sub.EXPECT().IsValidContract(dl.ContractID).Return(false, nil).AnyTimes()

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

	//manager.EXPECT().Commit(context.Background()).AnyTimes()
	assert.NoError(t, d.Sync(context.Background()))
	assert.Equal(t, dl.Vms, cp.Vms)
	assert.Equal(t, dl.Disks, cp.Disks)
	assert.Equal(t, dl.Qsfs, cp.Qsfs)
	assert.Equal(t, dl.Zdbs, cp.Zdbs)
	assert.Equal(t, dl.ContractID, cp.ContractID)
	assert.Equal(t, dl.NodeID, cp.NodeID)
}
