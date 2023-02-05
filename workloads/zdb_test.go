// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// ZDBWorkload for tests
var ZDBWorkload = ZDB{
	Name:        "test",
	Password:    "password",
	Public:      true,
	Size:        100,
	Description: "test des",
	Mode:        "user",
	//IPs:         ips,
	Port:      0,
	Namespace: "",
}

func TestZDB(t *testing.T) {
	var zdbWorkload gridtypes.Workload

	t.Run("test zdb to/from map", func(t *testing.T) {
		zdbFromMap := NewZDBFromSchema(ZDBWorkload.ToMap())
		assert.Equal(t, ZDBWorkload, zdbFromMap)
	})

	t.Run("test_zdb_from_workload", func(t *testing.T) {
		zdbWorkload = ZDBWorkload.ZosWorkload()

		res, err := json.Marshal(zos.ZDBResult{})
		assert.NoError(t, err)
		zdbWorkload.Result.Data = res

		zdbFromWorkload, err := NewZDBFromWorkload(&zdbWorkload)
		assert.NoError(t, err)
		assert.Equal(t, ZDBWorkload, zdbFromWorkload)
	})

	t.Run("test_zdb_functions", func(t *testing.T) {
		assert.Equal(t, ZDBWorkload.GetName(), "test")
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		zdbWorkloadCp := zdbWorkload
		zdbWorkloadCp.Result = gridtypes.Result{}

		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zdbWorkloadCp)

		workloadsMap2, err := ZDBWorkload.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
