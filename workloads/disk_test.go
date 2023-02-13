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
	SizeGB:      10,
	Description: "disk test description",
}

func TestDiskWorkload(t *testing.T) {
	var disk gridtypes.Workload

	t.Run("test_disk_from_map", func(t *testing.T) {
		diskFromMap := NewDiskFromMap(DiskWorkload.ToMap())
		assert.Equal(t, diskFromMap, DiskWorkload)
	})

	t.Run("test_disk_from_workload", func(t *testing.T) {
		disk = DiskWorkload.ZosWorkload()

		diskFromWorkload, err := NewDiskFromWorkload(&disk)
		assert.NoError(t, err)

		assert.Equal(t, diskFromWorkload, DiskWorkload)
	})
}

func TestDiskWorkloadFailures(t *testing.T) {
	disk := DiskWorkload.ZosWorkload()

	disk.Data = nil
	_, err := NewDiskFromWorkload(&disk)
	assert.Contains(t, err.Error(), "failed to get workload data")
}
