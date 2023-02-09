// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// QsfsWorkload for testing
var QsfsWorkload = QSFS{
	Name:                 "test",
	Description:          "test des",
	Cache:                2048,
	MinimalShards:        10,
	ExpectedShards:       20,
	RedundantGroups:      2,
	RedundantNodes:       5,
	MaxZDBDataDirSize:    10,
	EncryptionAlgorithm:  "AES",
	EncryptionKey:        "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
	CompressionAlgorithm: "snappy",
	Metadata: Metadata{
		Type:                "zdb",
		Prefix:              "test",
		EncryptionAlgorithm: "AES",
		EncryptionKey:       "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
		Backends: Backends{
			{Address: "1.1.1.1", Namespace: "test ns", Password: "password"},
		},
	},
	Groups: Groups{{Backends: Backends{
		{Address: "2.2.2.2", Namespace: "test ns2", Password: "password2"},
	}}},
}

func TestQsfsWorkload(t *testing.T) {
	var qsfs gridtypes.Workload

	t.Run("test new qsfs to/from map", func(t *testing.T) {
		qsfsFromMap := NewQSFSFromMap(QsfsWorkload.ToMap())
		assert.Equal(t, qsfsFromMap, QsfsWorkload)
	})

	t.Run("test_new_qsfs_from_workload", func(t *testing.T) {
		var err error
		qsfs, err = QsfsWorkload.ZosWorkload()
		assert.NoError(t, err)

		qsfsFromWorkload, err := NewQSFSFromWorkload(&qsfs)
		assert.NoError(t, err)
		assert.Equal(t, qsfsFromWorkload, QsfsWorkload)
	})

	t.Run("test_update_qsfs_from_workload", func(t *testing.T) {
		err := QsfsWorkload.UpdateFromWorkload(&qsfs)
		assert.NoError(t, err)
	})
}
