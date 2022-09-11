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

func TestGatewayNameDeployment(t *testing.T) {
	manager, _ := setup()
	backend := "http://162.205.240.240"
	expected := workloads.GatewayNameProxy{
		Name:           "testx",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "testx.Libra.Tfcloud.us",
	}

	err := expected.Stage(manager, 49)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	defer manager.CancelAll()
	result, err := loader.LoadGatewayNameFromGrid(manager, 49, "testx")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	err = manager.CancelAll()
	assert.NoError(t, err)
	expected = workloads.GatewayNameProxy{}
	wl, err := loader.LoadGatewayNameFromGrid(manager, 49, "testx")
	assert.Error(t, err)
	assert.Equal(t, reflect.DeepEqual(expected, wl), true)
}
