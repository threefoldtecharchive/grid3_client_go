// Package integration for integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestZDBDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	nodes, err := deployer.FilterNodes(tfPluginClient.GridProxyClient, nodeFilter)
	assert.NoError(t, err)

	nodeID := uint32(nodes[0].NodeID)

	zdb := workloads.ZDB{
		Name:        "testName",
		Password:    "password",
		Public:      true,
		Size:        10,
		Description: "test des",
		Mode:        zos.ZDBModeUser,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	dl := workloads.NewDeployment("zdb", nodeID, "", nil, "", nil, []workloads.ZDB{zdb}, nil, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	z, err := tfPluginClient.State.LoadZdbFromGrid(nodeID, zdb.Name, dl.Name)
	assert.NoError(t, err)
	assert.NotEmpty(t, z.IPs)
	assert.NotEmpty(t, z.Namespace)
	assert.NotEmpty(t, z.Port)

	z.IPs = nil
	z.Port = 0
	z.Namespace = ""
	assert.Equal(t, zdb, z)

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	_, err = tfPluginClient.State.LoadZdbFromGrid(nodeID, zdb.Name, dl.Name)
	assert.Error(t, err)
}
