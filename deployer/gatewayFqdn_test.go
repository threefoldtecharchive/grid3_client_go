package deployer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayFqdn(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)
	// print(tfPluginClient.Mnemonics)
	// publicKey := os.Getenv("PUBLICKEY")
	backend := "http://162.205.240.240/"
	gateway := workloads.GatewayFQDNProxy{
		Name:           "GatewayFqdn",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           "gatewayn.gridtesting.xyz",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = tfPluginClient.GatewayFQDNDeployer.Deploy(ctx, &gateway)
	assert.NoError(t, err)
}
