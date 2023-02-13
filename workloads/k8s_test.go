// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

var flist = "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist"

// K8sWorkload to be used in tests
var K8sWorkload = K8sNode{
	Name:          "test",
	Node:          0,
	DiskSize:      5,
	PublicIP:      false,
	PublicIP6:     false,
	Planetary:     false,
	Flist:         flist,
	FlistChecksum: "8ca00560e5c9633b9424ac1014957bf3",
	ComputedIP:    "",
	ComputedIP6:   "",
	YggIP:         "",
	IP:            "",
	CPU:           2,
	Memory:        1024,
}

func TestK8sNodeData(t *testing.T) {
	var cluster K8sCluster
	var k8sWorkloads []gridtypes.Workload

	t.Run("test k8s workload to/from map", func(t *testing.T) {
		k8sFromMap := NewK8sNodeFromMap(K8sWorkload.ToMap())
		assert.Equal(t, k8sFromMap, K8sWorkload)
	})

	t.Run("test_new_k8s_cluster", func(t *testing.T) {
		cluster = K8sCluster{
			Master:      &K8sWorkload,
			Workers:     []K8sNode{},
			Token:       "testToken",
			SSHKey:      "",
			NetworkName: "",
		}
	})

	t.Run("test_validate_names", func(t *testing.T) {
		err := cluster.ValidateNames()
		assert.NoError(t, err)
	})

	t.Run("test_validate_token", func(t *testing.T) {
		err := cluster.ValidateToken()
		assert.NoError(t, err)
	})

	t.Run("test_generate_k8s_workloads", func(t *testing.T) {
		k8sWorkloads = K8sWorkload.MasterZosWorkload(&cluster)

		assert.Equal(t, k8sWorkloads[0].Type, zos.ZMountType)
		assert.Equal(t, k8sWorkloads[1].Type, zos.ZMachineType)
		assert.Equal(t, len(k8sWorkloads), 2)
	})

	t.Run("test_k8s_from_workload", func(t *testing.T) {
		k8s := k8sWorkloads[1]
		k8sFromWorkload, err := NewK8sNodeFromWorkload(k8s, 0, 5, "", "")
		assert.NoError(t, err)

		k8sFromWorkload.IP = ""
		assert.Equal(t, k8sFromWorkload, K8sWorkload)
	})

	t.Run("test_generate_k8s_workloads_from_cluster", func(t *testing.T) {
		k8sWorkloads, err := cluster.ZosWorkloads()
		assert.NoError(t, err)
		assert.Equal(t, k8sWorkloads[0].Type, zos.ZMountType)
		assert.Equal(t, k8sWorkloads[1].Type, zos.ZMachineType)
		assert.Equal(t, len(k8sWorkloads), 2)
	})
}
