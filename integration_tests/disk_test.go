//go:build integration
// +build integration

// Package integration for integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/workloads"
)

func TestDiskDeployment(t *testing.T) {
	disk := workloads.Disk{
		Name:        "testName",
		Size:        20,
		Description: "disk test",
	}
	manager, _ := setup()
	err := manager.Stage(&disk, 13)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	err = manager.CancelAll()
	assert.NoError(t, err)
	/*
		result, err := loader.LoadDiskFromGrid(manager, 13, "testName")
		assert.Equal(t, disk, result)
		assert.NoError(t, err)
		err = manager.CancelAll()
		assert.NoError(t, err)
		_, err = loader.LoadDiskFromGrid(manager, 13, "testName")
		assert.Error(t, err)
	*/
}
