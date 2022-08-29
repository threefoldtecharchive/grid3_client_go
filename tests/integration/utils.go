//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/substrate-client"
)

func deployWorkload(t *testing.T, workload workloads.Workload, mnemonics string, twin uint32, nodeID uint32) deployer.DeploymentManager {
	identity, err := substrate.NewIdentityFromSr25519Phrase(mnemonics)
	assert.NoError(t, err)
	subManager := subi.NewManager("wss://tfchain.dev.grid.tf/ws")
	cl, err := client.NewProxyBus("https://gridproxy.dev.grid.tf/", twin, subManager, identity, true)
	assert.NoError(t, err)
	manager := deployer.NewDeploymentManager(identity, twin, map[uint32]uint64{}, proxy.NewClient("https://gridproxy.dev.grid.tf/"), client.NewNodeClientPool(cl), subManager)
	err = workload.Stage(manager, nodeID)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err = manager.Commit(ctx)
	assert.NoError(t, err)
	return manager
}
