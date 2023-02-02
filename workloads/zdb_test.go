// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
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

	res, err := json.Marshal(zos.ZDBResult{
		Namespace: "",
		IPs:       ips,
		Port:      0,
	})
	assert.NoError(t, err)

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
		Result: gridtypes.Result{
			Created: 1000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
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
		assert.Equal(t, zdb.ToMap(), zdbMap)
		assert.Equal(t, zdb.GetName(), "test")
	})

	t.Run("test_workload_from_zdb", func(t *testing.T) {
		zdbWorkloadCp := zdbWorkload
		zdbWorkloadCp.Result = gridtypes.Result{}

		workloadFromZDB := zdb.GenerateWorkload()
		assert.Equal(t, workloadFromZDB, zdbWorkloadCp)
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		zdbWorkloadCp := zdbWorkload
		zdbWorkloadCp.Result = gridtypes.Result{}

		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zdbWorkloadCp)

		workloadsMap2, err := zdb.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
