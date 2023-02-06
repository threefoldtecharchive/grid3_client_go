package deployer

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
)

func constructTestK8s(t *testing.T, mock bool) (
	K8sDeployer,
	*mocks.RMBMockClient,
	*mocks.MockSubstrateExt,
	*mocks.MockNodeClientGetter,
	*mocks.MockDeployer,
	*mocks.MockClient,
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	deployer := mocks.NewMockDeployer(ctrl)
	gridProxyCl := mocks.NewMockClient(ctrl)

	if mock {
		tfPluginClient.TwinID = twinID
		tfPluginClient.RMB = cl
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.GridProxyClient = gridProxyCl

		tfPluginClient.K8sDeployer.deployer = deployer
		tfPluginClient.K8sDeployer.tfPluginClient = &tfPluginClient

	}

	return tfPluginClient.K8sDeployer, cl, sub, ncPool, deployer, gridProxyCl
}
