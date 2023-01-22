// package loader to load different types, workloads from grid
package loader

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadZdbFromGrid(t *testing.T) {
	res, _ := json.Marshal(zos.ZDBResult{
		Namespace: "test name",
		IPs: []string{
			"1.1.1.1",
			"2.2.2.2",
		},
		Port: 5000,
	})
	zdbWl := gridtypes.Workload{
		Name:        gridtypes.Name("test"),
		Type:        zos.ZDBType,
		Description: "test des",
		Version:     0,
		Result: gridtypes.Result{
			Created: 1000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     100 * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode("user"),
			Password: "password",
			Public:   true,
		}),
	}
	zdb := workloads.ZDB{
		Name:        "test",
		Password:    "password",
		Public:      true,
		Size:        100,
		Description: "test des",
		Mode:        "user",
		Namespace:   "test name",
		IPs: []string{
			"1.1.1.1",
			"2.2.2.2",
		},
		Port: 5000,
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(zdbWl, nil)
		got, err := LoadZdbFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, zdb, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		zdbWlCp := zdbWl
		zdbWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(zdbWlCp, nil)
		_, err := LoadZdbFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		zdbWlCp := zdbWl
		zdbWlCp.Type = zos.GatewayNameProxyType
		zdbWlCp.Data = gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name: "name",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(zdbWlCp, nil)

		_, err := LoadZdbFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("invalid result data", func(t *testing.T) {
		zdbWlCp := zdbWl
		zdbWlCp.Result.Data = nil
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(zdbWlCp, nil)

		_, err := LoadZdbFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
