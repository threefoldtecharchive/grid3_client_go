// Package loader to load different types, workloads from grid
package loader

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadQsfsFromGrid(t *testing.T) {
	res, _ := json.Marshal(zos.QuatumSafeFSResult{
		Path:            "path",
		MetricsEndpoint: "endpoint",
	})
	k, _ := hex.DecodeString("4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af")
	qsfsWl := gridtypes.Workload{
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
		Result: gridtypes.Result{
			Created: 10000,
			State:   gridtypes.StateOk,
			Data:    res,
		},
	}
	qsfs := workloads.QSFS{
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
		Metadata: workloads.Metadata{
			Type:                "zdb",
			Prefix:              "test",
			EncryptionAlgorithm: "AES",
			EncryptionKey:       "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
			Backends: workloads.Backends{
				{Address: "1.1.1.1", Namespace: "test ns", Password: "password"},
			},
		},
		Groups: workloads.Groups{{Backends: workloads.Backends{
			{Address: "2.2.2.2", Namespace: "test ns2", Password: "password2"},
		}}},
		MetricsEndpoint: "endpoint",
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(qsfsWl, nil)
		got, err := LoadQsfsFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, qsfs, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		qsfsWlCp := qsfsWl
		qsfsWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(qsfsWlCp, nil)
		_, err := LoadQsfsFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		qsfsWlCp := qsfsWl
		qsfsWlCp.Type = zos.GatewayNameProxyType
		qsfsWlCp.Data = gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name: "name",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(qsfsWlCp, nil)

		_, err := LoadQsfsFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("invalid result data", func(t *testing.T) {
		qsfsWlCp := qsfsWl
		qsfsWlCp.Result.Data = nil
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(qsfsWlCp, nil)

		_, err := LoadQsfsFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
