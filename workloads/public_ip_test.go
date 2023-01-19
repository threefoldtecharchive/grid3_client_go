// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestPublicIPWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var publicIPWorkload gridtypes.Workload

	t.Run("test_construct_pub_ip_workload", func(t *testing.T) {
		publicIPWorkload = ConstructPublicIPWorkload("test", true, true)
		assert.NoError(t, publicIPWorkload.Type.Valid())
	})
}
