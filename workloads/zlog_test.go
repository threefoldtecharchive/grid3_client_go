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

	zmachineName := "test"

	urlHash := md5.Sum([]byte("output"))
	zlogWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name(zmachineName),
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

	zlogWorkload.Result.State = ""
	zlog := Zlog{Output: "output"}

	t.Run("test_zLogs_generate_workload", func(t *testing.T) {
		zosWorkload, err := zlog.GenerateWorkload(zmachineName)
		assert.NoError(t, err)
		assert.Equal(t, zosWorkload.Type, zos.ZLogsType)
		assert.Equal(t, zlogWorkload, zosWorkload)
	})

	t.Run("test_zLogs_from_deployment", func(t *testing.T) {
		zlogs := zlogs(&deployment, zmachineName)
		assert.Equal(t, zlogs, []Zlog{zlog})
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zlogWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := zlog.Stage(manager, nodeID, zmachineName)
		assert.NoError(t, err)
	})
}
