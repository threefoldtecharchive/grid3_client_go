// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestPublicIPWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var publicIPWorkload gridtypes.Workload

	t.Run("test_construct_pub_ip_workload", func(t *testing.T) {
		publicIPWorkload = ConstructPublicIPWorkload("test", true, true)
	})

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], publicIPWorkload)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := manager.SetWorkloads(workloadsMap)
		assert.NoError(t, err)
	})
}
