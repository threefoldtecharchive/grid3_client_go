// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// DiskWorkload to be used for tests
var DiskWorkload = Disk{
	Name:        "test",
	Size:        10,
	Description: "disk test description",
}

func TestDiskWorkload(t *testing.T) {
	var disk gridtypes.Workload

	t.Run("test_disk_from_map", func(t *testing.T) {
		diskFromSchema := NewDiskFromSchema(DiskWorkload.ToMap())
		assert.Equal(t, diskFromSchema, DiskWorkload)
	})

	t.Run("test_disk_name", func(t *testing.T) {
		assert.Equal(t, DiskWorkload.GetName(), DiskWorkload.Name)
	})

	t.Run("test_disk_from_workload", func(t *testing.T) {
		disk = DiskWorkload.ZosWorkload()

		diskFromWorkload, err := NewDiskFromWorkload(&disk)
		assert.NoError(t, err)

		assert.Equal(t, diskFromWorkload, DiskWorkload)
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], disk)

		workloadsMap2, err := DiskWorkload.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}

func TestDiskWorkloadFailures(t *testing.T) {
	disk := DiskWorkload.ZosWorkload()

	disk.Data = nil
	_, err := NewDiskFromWorkload(&disk)
	assert.Contains(t, err.Error(), "failed to get workload data")
}
