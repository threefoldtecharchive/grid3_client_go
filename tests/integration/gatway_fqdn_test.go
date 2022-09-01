package integration

import (
	"context"
	"fmt"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"

	"github.com/threefoldtech/grid3-go/workloads"

	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayFQDNDeployment(t *testing.T) {
	manager, apiClient := setup()
	backend := "http://185.206.122.36/24"
	expected := workloads.GatewayNameProxy{
		Name:           "TestGatwayFQDN",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "test.gname.alaa",
	}
	// fmt.Println(expected)

	err := expected.Stage(manager, 14)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	result, err := loader.LoadGatewayNameFromGrid(manager, 14, "TestGatewayName")
	assert.NoError(t, err)

	fmt.Println(result)
	fmt.Println(apiClient)

	assert.NotEmpty(t, result.Backends)
	assert.NotEmpty(t, result.FQDN)
	assert.NotEmpty(t, result.Name)
	assert.NotEmpty(t, result.TLSPassthrough)

	result.Backends = nil
	result.FQDN = ""
	result.Name = ""
	result.TLSPassthrough = false
	assert.Equal(t, expected, result)
	err = manager.CancelAll()
	assert.NoError(t, err)
	_, err = loader.LoadGatewayNameFromGrid(manager, 11, "testName")
	assert.Error(t, err)

}
