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

func TestGatewayFQDNDeployment(t *testing.T) {
	manager, _ := setup()
	backend := "http://162.205.240.240/"
	expected := workloads.GatewayFQDNProxy{
		Name:           "tf",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "gatewayn.gridtesting.xyz",
	}

	err := manager.Stage(&expected, 49)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err = manager.Commit(ctx)
	assert.NoError(t, err)

	err = manager.CancelAll()
	assert.NoError(t, err)
	result, err := loader.LoadGatewayFqdnFromGrid(manager, 49, "tf")
	assert.NoError(t, err)

	assert.Equal(t, expected, result)

	err = manager.CancelAll()
	assert.NoError(t, err)
	expected = workloads.GatewayFQDNProxy{}

	wl, err := loader.LoadGatewayFqdnFromGrid(manager, 49, "tf")
	assert.Error(t, err)
	assert.Equal(t, expected, wl)

}
