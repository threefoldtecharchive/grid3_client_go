// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// ZlogWorkload for tests
var ZlogWorkload = Zlog{
	Zmachine: "test",
	Output:   "output",
}

func TestZLog(t *testing.T) {
	zlogWorkload := ZlogWorkload.ZosWorkload()
	zlogWorkload.Result.State = "ok"

	deployment := NewGridDeployment(1, []gridtypes.Workload{zlogWorkload})

	t.Run("test_zLogs_from_deployment", func(t *testing.T) {
		zlogs := zlogs(&deployment, ZlogWorkload.Zmachine)
		assert.Equal(t, zlogs, []Zlog{ZlogWorkload})
	})
}
