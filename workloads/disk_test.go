// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestDiskWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var disk Disk

	diskName := "test"
	diskDescription := "test description"

	diskMap := map[string]interface{}{
		"name":        diskName,
		"size":        100,
		"description": diskDescription,
	}

	diskWorkload := gridtypes.Workload{
		Name:        gridtypes.Name(diskName),
		Version:     0,
		Type:        zos.ZMountType,
		Description: diskDescription,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: 100 * gridtypes.Gigabyte,
		}),
	}

	t.Run("test_disk_from_map", func(t *testing.T) {
		disk = NewDiskFromSchema(diskMap)

	})

	t.Run("test_disk_functions", func(t *testing.T) {
		assert.Equal(t, disk.ToMap(), diskMap)
		assert.Equal(t, disk.GetName(), diskName)
	})

	t.Run("test_disk_from_workload", func(t *testing.T) {
		diskFromWorkload, err := NewDiskFromWorkload(&diskWorkload)
		assert.NoError(t, err)

		assert.Equal(t, disk, diskFromWorkload)
	})

	t.Run("test_workload_from_disk", func(t *testing.T) {
		workloadFromDisk, err := disk.GenerateWorkloads()
		assert.NoError(t, err)
		assert.Equal(t, diskWorkload, workloadFromDisk[0])
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], diskWorkload)

		workloadsMap2, err := disk.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
