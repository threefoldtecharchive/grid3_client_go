// Package loader to load different types, workloads from grid
package loader

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadK8sFromGrid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := mocks.NewMockDeploymentManager(ctrl)

	flist := "https://hub.grid.tf/tf-official-apps/base:latest.flist"
	flistCheckSum, err := workloads.GetFlistChecksum(flist)
	assert.NoError(t, err)

	res, _ := json.Marshal(zos.ZMachineResult{
		IP:    "1.1.1.1",
		YggIP: "203:8b0b:5f3e:b859:c36:efdf:ab6e:50cc",
	})

	master := workloads.K8sNodeData{
		Name:          "test",
		Node:          1,
		DiskSize:      0,
		Flist:         flist,
		FlistChecksum: flistCheckSum,
		PublicIP:      false,
		Planetary:     true,
		CPU:           1,
		Memory:        8,
		YggIP:         "203:8b0b:5f3e:b859:c36:efdf:ab6e:50cc",
		IP:            "1.1.1.1",
	}

	var Workers []workloads.K8sNodeData
	cluster := workloads.K8sCluster{
		Master:      &master,
		Workers:     Workers,
		Token:       "",
		SSHKey:      "",
		NetworkName: "",
	}

	k8sWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test"),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: flist,
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name("test_network"),
						IP:      net.ParseIP("1.1.1.1"),
					},
				},
				Planetary: true,
			},
			Size: 100,
			ComputeCapacity: zos.MachineCapacity{
				CPU:    1,
				Memory: 8 * gridtypes.Megabyte,
			},
			Mounts:     []zos.MachineMount{},
			Entrypoint: "",
			Env:        map[string]string{},
			Corex:      false,
		}),
		Result: gridtypes.Result{
			Created: 5000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
	}

	dl := gridtypes.Deployment{
		Workloads: []gridtypes.Workload{k8sWorkload},
	}

	t.Run("success", func(t *testing.T) {
		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(k8sWorkload, nil)

		got, err := LoadK8sFromGrid(manager, map[uint32]string{1: "test"}, map[uint32][]string{})
		assert.NoError(t, err)
		assert.Equal(t, cluster, got)
	})

	t.Run("invalid type", func(t *testing.T) {
		k8sWorkloadCp := k8sWorkload
		k8sWorkloadCp.Type = "invalid"

		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(k8sWorkloadCp, nil)

		_, err := LoadK8sFromGrid(manager, map[uint32]string{1: "test"}, map[uint32][]string{})
		assert.Error(t, err)
	})

	t.Run("wrong workload data", func(t *testing.T) {
		k8sWorkloadCp := k8sWorkload
		k8sWorkloadCp.Type = zos.ZMachineType
		k8sWorkloadCp.Data = gridtypes.MustMarshal(zos.ZMachine{
			FList: "",
		})

		manager.EXPECT().GetDeployment(uint32(1)).Return(dl, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(k8sWorkloadCp, nil)

		_, err := LoadK8sFromGrid(manager, map[uint32]string{1: "test"}, map[uint32][]string{})
		assert.Error(t, err)
	})
}
