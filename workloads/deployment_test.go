// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestNewDeployment(t *testing.T) {
	var twinID uint32 = 11

	d := NewDeployment(twinID, []gridtypes.Workload{})

	assert.Equal(t, d.Version, uint32(0))
	assert.Equal(t, d.TwinID, twinID)
	assert.Equal(t, d.Workloads, []gridtypes.Workload{})
	assert.Equal(t, d.ContractID, uint64(0))
}
