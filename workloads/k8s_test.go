// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestK8sNodeData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var k8s K8sNodeData
	var cluster K8sCluster
	var k8sWorkloads []gridtypes.Workload

	flist := "https://hub.grid.tf/tf-official-apps/base:latest.flist"
	flistCheckSum, err := GetFlistChecksum(flist)
	assert.NoError(t, err)

	k8sMap := map[string]interface{}{
		"name":           "test",
		"node":           1,
		"disk_size":      100,
		"publicip":       false,
		"publicip6":      false,
		"planetary":      false,
		"flist":          flist,
		"flist_checksum": flistCheckSum,
		"computedip":     "",
		"computedip6":    "",
		"ygg_ip":         "",
		"ip":             "<nil>",
		"cpu":            2,
		"memory":         8,
	}

	k8sWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test"),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: flist,
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{IP: net.IP("")},
				},
			},
			Size: 100,
			ComputeCapacity: zos.MachineCapacity{
				CPU:    2,
				Memory: 8 * gridtypes.Megabyte,
			},
			Mounts:     []zos.MachineMount{},
			Entrypoint: "",
			Env:        map[string]string{},
			Corex:      false,
		}),
	}

	t.Run("test_new_k8s_node_data", func(t *testing.T) {
		k8s = NewK8sNodeData(k8sMap)

	})

	t.Run("test_k8s_from_workload", func(t *testing.T) {
		k8sFromWorkload, err := NewK8sNodeDataFromWorkload(k8sWorkload, 1, 100, "", "")
		assert.NoError(t, err)

		assert.Equal(t, k8s, k8sFromWorkload)
	})

	t.Run("test_k8s_node_data_dictify", func(t *testing.T) {
		assert.Equal(t, k8s.Dictify(), k8sMap)
	})

	t.Run("test_new_k8s_cluster", func(t *testing.T) {
		cluster = K8sCluster{
			Master:      &k8s,
			Workers:     []K8sNodeData{},
			Token:       "token",
			SSHKey:      "",
			NetworkName: "",
		}
	})

	t.Run("test_validate_names", func(t *testing.T) {
		err := cluster.ValidateNames(context.Background())
		assert.NoError(t, err)
	})

	t.Run("test_validate_token", func(t *testing.T) {
		err := cluster.ValidateToken(context.Background())
		assert.NoError(t, err)
	})

	t.Run("test_generate_k8s_workloads", func(t *testing.T) {
		k8sWorkloads = k8s.GenerateK8sWorkload(&cluster, false)

		assert.Equal(t, k8sWorkloads[0].Type, zos.ZMountType)
		assert.Equal(t, k8sWorkloads[1].Type, zos.ZMachineType)
		assert.Equal(t, len(k8sWorkloads), 2)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[cluster.Master.Node] = append(workloadsMap[cluster.Master.Node], k8sWorkloads...)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := cluster.Stage(context.Background(), manager)
		assert.NoError(t, err)
	})
}
