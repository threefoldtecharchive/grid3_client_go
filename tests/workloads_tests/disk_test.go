package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	mock_deployer "github.com/threefoldtech/grid3-go/tests/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestDiskStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
	wlMap := map[uint32][]gridtypes.Workload{}
	wlMap[1] = append(wlMap[1], diskWl)
	manager := mock_deployer.NewMockDeploymentManager(ctrl)
	manager.EXPECT().SetWorkloads(gomock.Eq(wlMap)).Return(nil)
	err := disk.Stage(manager, 1)
	assert.NoError(t, err)
}
