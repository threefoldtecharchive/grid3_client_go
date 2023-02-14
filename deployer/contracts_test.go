// Package deployer for grid deployer
package deployer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContractsGetter(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	_, err = tfPluginClient.ContractsGetter.ListContractsByTwinID([]string{"Created, GracePeriod"})
	assert.NoError(t, err)

	contracts, err := tfPluginClient.ContractsGetter.ListContractsOfProjectName("badName")
	assert.Empty(t, contracts.NameContracts)
	assert.Empty(t, contracts.NodeContracts)
	assert.Empty(t, contracts.RentContracts)
	assert.NoError(t, err)
}
