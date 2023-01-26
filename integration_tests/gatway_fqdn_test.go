//go:build integration
// +build integration

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

func TestGatewayFQDNDeployment(t *testing.T) {
	dlManager, _ := setup()
	backend := "http://162.205.240.240/"
	expected := workloads.GatewayFQDNProxy{
		Name:           "tf",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "gatewayn.gridtesting.xyz",
	}

	err := dlManager.Stage(&expected, 49)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err = dlManager.Commit(ctx)
	assert.NoError(t, err)

	err = dlManager.CancelAll()
	assert.NoError(t, err)
	result, err := manager.LoadGatewayFqdnFromGrid(dlManager, 49, "tf")
	assert.NoError(t, err)

	assert.Equal(t, expected, result)

	err = dlManager.CancelAll()
	assert.NoError(t, err)
	expected = workloads.GatewayFQDNProxy{}

	wl, err := manager.LoadGatewayFqdnFromGrid(dlManager, 49, "tf")
	assert.Error(t, err)
	assert.Equal(t, expected, wl)

}
