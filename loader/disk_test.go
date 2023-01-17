package loader

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadDiskFromGrid(t *testing.T) {
	disk := workloads.Disk{
		Name:        "test",
		Size:        100,
		Description: "test des",
	}
	diskWl := gridtypes.Workload{
		Name:        gridtypes.Name("test"),
		Version:     0,
		Type:        zos.ZMountType,
		Description: "test des",
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: 100 * gridtypes.Gigabyte,
		}),
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(diskWl, nil)
		got, err := LoadDiskFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, disk, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		diskWlCp := diskWl
		diskWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(diskWlCp, nil)
		_, err := LoadDiskFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		diskWlCp := diskWl
		diskWlCp.Type = zos.GatewayNameProxyType
		diskWlCp.Data = gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name: "name",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(diskWlCp, nil)

		_, err := LoadDiskFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
