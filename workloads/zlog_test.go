// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"crypto/md5"
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	urlHash := md5.Sum([]byte("output"))
	zlogWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name("test"),
			Output:   "output",
		}),
	}

	zlogWorkload.Result.State = "ok"
	deployment := gridtypes.Deployment{
		Version:   0,
		TwinID:    1,
		Workloads: []gridtypes.Workload{zlogWorkload},
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: 1,
					Weight: 1,
				},
			},
		},
	}

	zlog := Zlog{Output: "output"}

	t.Run("test_zLogs_generate_workload", func(t *testing.T) {
		zosWorkload := zlog.GenerateWorkload("test")
		assert.Equal(t, zosWorkload.Type, zos.ZLogsType)
	})

	t.Run("test_zLogs_from_deployment", func(t *testing.T) {
		zlogs := zlogs(&deployment, "test")
		assert.Equal(t, zlogs, []Zlog{zlog})
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zlogWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := manager.SetWorkloads(workloadsMap)
		assert.NoError(t, err)
	})
}
