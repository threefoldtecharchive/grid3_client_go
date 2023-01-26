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
	deploymentManager, _ := setup()
	err := deploymentManager.Stage(&disk, nodeID)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	err = deploymentManager.Commit(ctx)
	assert.NoError(t, err)

	err = deploymentManager.CancelAll()
	assert.NoError(t, err)

	result, err := manager.LoadDiskFromGrid(deploymentManager, 13, "testName")
	assert.Equal(t, disk, result)
	assert.NoError(t, err)
	err = deploymentManager.CancelAll()
	assert.NoError(t, err)
	_, err = manager.LoadDiskFromGrid(deploymentManager, 13, "testName")
	assert.Error(t, err)

}
