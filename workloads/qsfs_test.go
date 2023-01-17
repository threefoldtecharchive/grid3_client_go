// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZdbBackendsAndGroups(t *testing.T) {
	backend := Backend{
		Address: "1.1.1.1", Namespace: "test ns", Password: "password",
	}

	backends := []Backend{backend}

	backendMap := map[string]interface{}{
		"address": "1.1.1.1", "namespace": "test ns", "password": "password",
	}

	group := Group{
		Backends: backends,
	}

	groupMap := map[string]interface{}{
		"backends": []interface{}{
			backendMap,
		},
	}

	t.Run("test_zos_group", func(t *testing.T) {
		zdbGroup := group.zosGroup()
		assert.Equal(t, len(zdbGroup.Backends), 1)
		assert.Equal(t, zdbGroup.Backends[0], zos.ZdbBackend(backend))

		assert.Equal(t, groupMap, group.Dictify())
	})

	t.Run("test_zos_groups", func(t *testing.T) {
		groups := Groups{group}
		zdbGroups := groups.zosGroups()

		assert.Equal(t, len(zdbGroups), 1)
		assert.Equal(t, zdbGroups[0].Backends[0], zos.ZdbBackend(backend))

		assert.Equal(t, groups, GroupsFromZos(zdbGroups))

		assert.Equal(t, []interface{}{groupMap}, groups.Listify())
	})

	t.Run("test_zos_backend", func(t *testing.T) {
		zosBackend := backend.zosBackend()
		assert.Equal(t, zosBackend, zos.ZdbBackend(backend))

		assert.Equal(t, backendMap, backend.Dictify())
	})

	t.Run("test_zos_backends", func(t *testing.T) {
		backends := Backends{backend}
		zosBackends := backends.zosBackends()

		assert.Equal(t, len(zosBackends), 1)
		assert.Equal(t, zosBackends[0], zos.ZdbBackend(backend))

		assert.Equal(t, backends, BackendsFromZos(zosBackends))

		assert.Equal(t, []interface{}{backendMap}, backends.Listify())

		assert.Equal(t, backends, getBackends([]interface{}{backendMap}))
	})
}

func TestMetaData(t *testing.T) {
	backend := Backend{
		Address: "1.1.1.1", Namespace: "test ns", Password: "password",
	}

	backendMap := map[string]interface{}{
		"address": "1.1.1.1", "namespace": "test ns", "password": "password",
	}

	backends := []Backend{backend}

	metadata := Metadata{
		Type:                "",
		Prefix:              "",
		EncryptionAlgorithm: "",
		EncryptionKey:       "",
		Backends:            backends,
	}

	metadataMap := map[string]interface{}{
		"type":                 "",
		"prefix":               "",
		"encryption_algorithm": "",
		"encryption_key":       "",
		"backends":             []interface{}{backendMap},
	}

	assert.Equal(t, metadataMap, metadata.Dictify())
}

func TestQSFSWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var qsfsMap map[string]interface{}

	qsfs := QSFS{
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
	k, _ := hex.DecodeString("4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af")

	qsfsWorkload := gridtypes.Workload{
		Version:     0,
		Name:        gridtypes.Name("test"),
		Type:        zos.QuantumSafeFSType,
		Description: "test des",
		Data: gridtypes.MustMarshal(zos.QuantumSafeFS{
			Cache: 2048 * gridtypes.Megabyte,
			Config: zos.QuantumSafeFSConfig{
				MinimalShards:     10,
				ExpectedShards:    20,
				RedundantGroups:   2,
				RedundantNodes:    5,
				MaxZDBDataDirSize: 10,
				Encryption: zos.Encryption{
					Algorithm: zos.EncryptionAlgorithm("AES"),
					Key:       zos.EncryptionKey(k),
				},
				Meta: zos.QuantumSafeMeta{
					Type: "zdb",
					Config: zos.QuantumSafeConfig{
						Prefix: "test",
						Encryption: zos.Encryption{
							Algorithm: zos.EncryptionAlgorithm("AES"),
							Key:       zos.EncryptionKey(k),
						},
						Backends: []zos.ZdbBackend{
							{Address: "1.1.1.1", Namespace: "test ns", Password: "password"},
						},
					},
				},
				Groups: []zos.ZdbGroup{{Backends: []zos.ZdbBackend{
					{Address: "2.2.2.2", Namespace: "test ns2", Password: "password2"},
				}}},
				Compression: zos.QuantumCompression{
					Algorithm: "snappy",
				},
			},
		}),
	}

	t.Run("test_schema_from_qsfs", func(t *testing.T) {
		qsfsMap = qsfs.Dictify()
	})

	t.Run("test_new_qsfs_from_schema", func(t *testing.T) {
		qsfsFromSchema := NewQSFSFromSchema(qsfsMap)
		assert.Equal(t, qsfsFromSchema, qsfs)
	})

	t.Run("test_new_qsfs_from_workload", func(t *testing.T) {
		qsfsFromWorkload, err := NewQSFSFromWorkload(&qsfsWorkload)
		assert.NoError(t, err)
		assert.Equal(t, qsfsFromWorkload, qsfs)
	})

	t.Run("test_qsfs_zos_workload", func(t *testing.T) {
		workloadFromQSFS, err := qsfs.ZosWorkload()
		assert.NoError(t, err)
		assert.Equal(t, workloadFromQSFS, qsfsWorkload)
	})

	t.Run("test_update_qsfs_from_workload", func(t *testing.T) {
		err := qsfs.UpdateFromWorkload(&qsfsWorkload)
		assert.NoError(t, err)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], qsfsWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := manager.SetWorkloads(workloadsMap)
		assert.NoError(t, err)
	})
}
