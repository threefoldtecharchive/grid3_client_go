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
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	backend := "http://162.205.240.240/"
	expected := workloads.GatewayFQDNProxy{
		Name:           "tf",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "gatewayn.gridtesting.xyz",
	}

	err = tfPluginClient.Manager.Stage(&expected, 49)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err = tfPluginClient.Manager.Commit(ctx)
	assert.NoError(t, err)

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)
	result, err := manager.LoadGatewayFqdnFromGrid(tfPluginClient.Manager, 49, "tf")
	assert.NoError(t, err)

	assert.Equal(t, expected, result)

	err = tfPluginClient.Manager.CancelAll()
	assert.NoError(t, err)
	expected = workloads.GatewayFQDNProxy{}

	wl, err := manager.LoadGatewayFqdnFromGrid(tfPluginClient.Manager, 49, "tf")
	assert.Error(t, err)
	assert.Equal(t, expected, wl)

}
