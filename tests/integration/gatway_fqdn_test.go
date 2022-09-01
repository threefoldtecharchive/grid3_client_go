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
	backend := "http://185.206.122.36"
	expected := workloads.GatewayFQDNProxy{
		Name:           "tfq",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "test.gname.alaa",
	}

	err := expected.Stage(manager, 14)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	result, err := loader.LoadGatewayFqdnFromGrid(manager, 14, "tfq")
	assert.NoError(t, err)

	assert.Equal(t, expected.Backends, result.Backends)
	assert.Equal(t, expected.Name, result.Name)
	assert.Equal(t, expected.TLSPassthrough, result.TLSPassthrough)

	err = manager.CancelAll()
	assert.NoError(t, err)
	_, err = loader.LoadGatewayFqdnFromGrid(manager, 14, "tfq")
	assert.Error(t, err)

}
