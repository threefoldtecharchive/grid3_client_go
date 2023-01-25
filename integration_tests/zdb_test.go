// Package integration for integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/manager"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZDBDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	zdb := workloads.ZDB{
		Name:        "testName",
		Password:    "password",
		Public:      true,
		Size:        20,
		Description: "test des",
		Mode:        zos.ZDBModeUser,
	}

	err = tfPluginClient.Manager.Stage(&zdb, 13)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	err = tfPluginClient.Manager.Commit(ctx)
	assert.NoError(t, err)

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)

	result, err := manager.LoadZdbFromGrid(tfPluginClient.Manager, 13, "testName")
	assert.NoError(t, err)
	assert.NotEmpty(t, result.IPs)
	assert.NotEmpty(t, result.Namespace)
	assert.NotEmpty(t, result.Port)
	result.IPs = nil
	result.Port = 0
	result.Namespace = ""
	assert.Equal(t, zdb, result)
	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)
	_, err = manager.LoadZdbFromGrid(tfPluginClient.Manager, 13, "testName")
	assert.Error(t, err)
}
