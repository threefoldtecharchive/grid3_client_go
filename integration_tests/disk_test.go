// Package integration for integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/manager"
	"github.com/threefoldtech/grid3-go/workloads"
)

func TestDiskDeployment(t *testing.T) {
	nodeID := uint32(30)

	disk := workloads.Disk{
		Name:        "testName",
		Size:        20,
		Description: "disk test",
	}
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	err = tfPluginClient.Manager.Stage(&disk, nodeID)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	err = tfPluginClient.Manager.Commit(ctx)
	assert.NoError(t, err)

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)

	result, err := manager.LoadDiskFromGrid(tfPluginClient.Manager, 13, "testName")
	assert.Equal(t, disk, result)
	assert.NoError(t, err)
	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)
	_, err = manager.LoadDiskFromGrid(tfPluginClient.Manager, 13, "testName")
	assert.Error(t, err)

}
