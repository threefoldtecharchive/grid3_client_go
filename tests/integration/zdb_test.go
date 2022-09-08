package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZDBDeployment(t *testing.T) {
	zdb := workloads.ZDB{
		Name:        "testName",
		Password:    "password",
		Public:      true,
		Size:        20,
		Description: "test des",
		Mode:        zos.ZDBModeUser,
	}
	manager, _ := setup()
	err := zdb.Stage(manager, 13)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	result, err := loader.LoadZdbFromGrid(manager, 13, "testName")
	assert.NoError(t, err)
	assert.NotEmpty(t, result.IPs)
	assert.NotEmpty(t, result.Namespace)
	assert.NotEmpty(t, result.Port)
	result.IPs = nil
	result.Port = 0
	result.Namespace = ""
	assert.Equal(t, zdb, result)
	err = manager.CancelAll()
	assert.NoError(t, err)
	_, err = loader.LoadZdbFromGrid(manager, 13, "testName")
	assert.Error(t, err)
}
