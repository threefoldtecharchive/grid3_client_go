//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZDBDeployment(t *testing.T) {
	expected := workloads.ZDB{
		Name:        "testName",
		Password:    "password",
		Public:      true,
		Size:        20,
		Description: "test des",
		Mode:        zos.ZDBModeUser,
	}
	manager := deployWorkload(t, &expected, "panther traffic explain chest source kiss elegant sense resist travel make drip", 192, 13)
	result, err := loader.LoadZdbFromGrid(manager, 13, "testName")
	assert.NoError(t, err)
	// TODO: try connecting to zdb
	assert.NotEmpty(t, result.IPs)
	assert.NotEmpty(t, result.Namespace)
	assert.NotEmpty(t, result.Port)
	result.IPs = nil
	result.Port = 0
	result.Namespace = ""
	assert.Equal(t, expected, result)
	err = manager.CancelAll()
	assert.NoError(t, err)
	_, err = loader.LoadZdbFromGrid(manager, 13, "testName")
	assert.Error(t, err)
}
