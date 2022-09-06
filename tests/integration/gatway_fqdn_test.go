package integration

import (
	"context"
	"reflect"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"

	"github.com/threefoldtech/grid3-go/workloads"

	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayFQDNDeployment(t *testing.T) {
	manager, _ := setup()
	backend := "http://185.206.122.36"
	expected := workloads.GatewayFQDNProxy{
		Name:           "tf",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "gname.gridtesting.xyz",
	}

	err := expected.Stage(manager, 14)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = manager.Commit(ctx)
	defer manager.CancelAll()
	assert.NoError(t, err)
	result, err := loader.LoadGatewayFqdnFromGrid(manager, 14, "tf")
	assert.NoError(t, err)

	assert.Equal(t, reflect.DeepEqual(expected, result), true)

	err = manager.CancelAll()
	assert.NoError(t, err)
	expected = workloads.GatewayFQDNProxy{}

	wl, err := loader.LoadGatewayFqdnFromGrid(manager, 14, "tf")
	assert.Error(t, err)
	assert.Equal(t, reflect.DeepEqual(expected, wl), true)

}
