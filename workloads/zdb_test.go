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

func TestZDBStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := mocks.NewMockDeploymentManager(ctrl)

	zdb := ZDB{
		Name:        "test",
		Password:    "password",
		Public:      true,
		Size:        100,
		Description: "test des",
		Mode:        "user",
	}
	zdbWl := gridtypes.Workload{
		Name:        gridtypes.Name("test"),
		Type:        zos.ZDBType,
		Description: "test des",
		Version:     0,
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     100 * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode("user"),
			Password: "password",
			Public:   true,
		}),
	}
	wlMap := map[uint32][]gridtypes.Workload{}
	wlMap[1] = append(wlMap[1], zdbWl)
	manager.EXPECT().SetWorkloads(gomock.Eq(wlMap)).Return(nil)
	err := zdb.Stage(manager, 1)
	assert.NoError(t, err)
}
