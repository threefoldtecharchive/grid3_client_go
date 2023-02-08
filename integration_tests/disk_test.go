// Package integration for integration tests
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

func TestDiskDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	filter := NodeFilter{
		Status: "up",
		SRU:    10,
	}
	nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	nodeID := nodeIDs[0]

	disk := workloads.Disk{
		Name:        "testName",
		SizeGP:      10,
		Description: "disk test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	dl := workloads.NewDeployment("disk", nodeID, "", nil, "", []workloads.Disk{disk}, nil, nil, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	resDisk, err := tfPluginClient.StateLoader.LoadDiskFromGrid(nodeID, disk.Name)
	assert.NoError(t, err)
	assert.Equal(t, disk, resDisk)

	// cancel all
	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	_, err = tfPluginClient.StateLoader.LoadDiskFromGrid(nodeID, disk.Name)
	assert.Error(t, err)
}
