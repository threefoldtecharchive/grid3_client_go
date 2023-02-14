// Package integration for integration tests
package integration

/*
import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"

	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestGatewayFQDNDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey, privateKey, err := GenerateSSHKeyPair()
	assert.NoError(t, err)

	filter := NodeFilter{
		Status:  "up",
		Gateway: true,
	}
	nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
	assert.NoError(t, err)

	nodeID := nodeIDs[0]
	gwNodeID := nodeIDs[1]

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		Entrypoint: "/sbin/zinit init",
		EnvVars: map[string]string{
			"SSH_KEY": publicKey,
		},
		NetworkName: network.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("vm", nodeID, "", nil, network.Name, nil, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.State.LoadVMFromGrid(nodeID, vm.Name)
	assert.NoError(t, err)
	assert.True(t, TestConnection(v.YggIP, "22"))

	backend := fmt.Sprintf("http://[%s]:9000", v.YggIP)
	fqdn := "" //"test.hamada.grid.tf" //"hamada1.3x0.me"
	gw := workloads.GatewayFQDNProxy{
		NodeID:         gwNodeID,
		Name:           "test",
		TLSPassthrough: false,
		Backends:       []zos.Backend{zos.Backend(backend)},
		FQDN:           fqdn,
	}

	err = tfPluginClient.GatewayFQDNDeployer.Deploy(ctx, &gw)
	assert.NoError(t, err)

	result, err := tfPluginClient.State.LoadGatewayFqdnFromGrid(nodeID, gw.Name)
	assert.NoError(t, err)
	assert.Equal(t, gw, result)

	_, err = RemoteRun("root", v.YggIP, "apk add python3; python3 -m http.server 9000 --bind :: &> /dev/null &", privateKey)
	assert.NoError(t, err)

	time.Sleep(3 * time.Second)

	response, err := http.Get(fmt.Sprintf("http://%s", gw.FQDN))
	assert.NoError(t, err)

	body, err := io.ReadAll(response.Body)
	assert.NoError(t, err)
	if body != nil {
		defer response.Body.Close()
	}
	assert.Contains(t, string(body), "Directory listing for")

	// cancel all
	err = tfPluginClient.GatewayFQDNDeployer.Cancel(ctx, &gw)
	assert.NoError(t, err)

	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)

	_, err = tfPluginClient.State.LoadGatewayFqdnFromGrid(nodeID, gw.Name)
	assert.Error(t, err)
}
*/
