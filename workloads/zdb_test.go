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

func TestZDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var ips []string

	zdb := ZDB{
		Name:        "test",
		Password:    "password",
		Public:      true,
		Size:        100,
		Description: "test des",
		Mode:        "user",
		IPs:         ips,
		Port:        0,
		Namespace:   "",
	}

	zdbMap := map[string]interface{}{
		"name":        "test",
		"size":        100,
		"description": "test des",
		"password":    "password",
		"public":      true,
		"mode":        "user",
		"ips":         ips,
		"port":        0,
		"namespace":   "",
	}

	zdbWorkload := gridtypes.Workload{
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

	t.Run("test_zdb_from_map", func(t *testing.T) {
		zdbFromMap := NewZDBFromSchema(zdbMap)
		assert.Equal(t, zdb, zdbFromMap)

	})

	t.Run("test_zdb_from_workload", func(t *testing.T) {
		zdbFromWorkload, err := NewZDBFromWorkload(&zdbWorkload)
		assert.NoError(t, err)
		assert.Equal(t, zdb, zdbFromWorkload)
	})

	t.Run("test_zdb_functions", func(t *testing.T) {
		assert.Equal(t, zdb.Dictify(), zdbMap)
		assert.Equal(t, zdb.GetName(), "test")
		assert.Equal(t, zdb.GenerateZDBWorkload(), zdbWorkload)
	})

	t.Run("test_workload_from_zdb", func(t *testing.T) {
		workloadFromZDB, err := zdb.GenerateWorkloadFromZDB()
		assert.NoError(t, err)
		assert.Equal(t, workloadFromZDB, zdbWorkload)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zdbWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := zdb.Stage(manager, nodeID)
		assert.NoError(t, err)
	})
}