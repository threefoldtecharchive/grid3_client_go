//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
)

func TestLoadDiskFromGrid(t *testing.T) {
	expected := workloads.Disk{
		Name:        "testName",
		Size:        20,
		Description: "disk test",
	}
	manager := deployWorkload(t, &expected, "panther traffic explain chest source kiss elegant sense resist travel make drip", 192, 13)
	result, err := loader.LoadDiskFromGrid(manager, 13, "testName")
	assert.Equal(t, expected, result)
	assert.NoError(t, err)
	err = manager.CancelAll()
	assert.NoError(t, err)
	_, err = loader.LoadDiskFromGrid(manager, 13, "testName")
	assert.Error(t, err)
}
