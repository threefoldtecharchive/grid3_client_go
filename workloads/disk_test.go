// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
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
		disk = ConvertsMapIntoDisk(diskMap)

	})

	t.Run("test_disk_functions", func(t *testing.T) {
		assert.Equal(t, disk.Dictify(), diskMap)
		assert.Equal(t, disk.GetName(), diskName)
		assert.Equal(t, disk.GenerateDiskWorkload(), diskWorkload)
	})

	t.Run("test_disk_from_workload", func(t *testing.T) {
		diskFromWorkload, err := NewDiskFromWorkload(&diskWorkload)
		assert.NoError(t, err)

		assert.Equal(t, disk, diskFromWorkload)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], diskWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := manager.SetWorkloads(workloadsMap)
		assert.NoError(t, err)
	})
}
