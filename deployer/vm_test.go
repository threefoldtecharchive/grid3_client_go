// Package deployer for deployer tests
package deployer

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"os"
	"os/exec"
	"testing"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func setup() (TFPluginClient, error) {
	if _, err := os.Stat("../.env"); !errors.Is(err, os.ErrNotExist) {
		err := godotenv.Load("../.env")
		if err != nil {
			return TFPluginClient{}, err
		}
	}

	mnemonics := os.Getenv("MNEMONICS")
	log.Printf("mnemonics: %s", mnemonics)

	network := os.Getenv("NETWORK")
	log.Printf("network: %s", network)

	SSHKeys()

	return NewTFPluginClient(mnemonics, "sr25519", network, "", "", true, "", true)
}

func SSHKeys() {
	err := os.Mkdir("/tmp/.ssh", 0755)
	if err != nil {
		fmt.Println(err)
	}

	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-f", "/tmp/.ssh/id_rsa", "-q")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(stdout))

	privateKey, err := os.ReadFile("/tmp/.ssh/id_rsa")
	if err != nil {
		log.Fatal(err)
	}

	publicKey, e := os.ReadFile("/tmp/.ssh/id_rsa.pub")
	if e != nil {
		log.Fatal(err)
	}

	os.Setenv("PUBLICKEY", string(publicKey))
	os.Setenv("PRIVATEKEY", string(privateKey))
}

func TestVMDeployment(t *testing.T) {
	tfPluginClient, err := setup()
	assert.NoError(t, err)

	publicKey := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDNFMdYHGcGqWsE7H1eqsWaXwOQQQrh6bYWsKKGa7KswNa8BhyEK9bjxEs13LvIVPUckn/wVVqlH0qFAc8JjBRmSjGdDjyZIvawOIyDX/Jr0fPAyS3e8eL+FvuJVW1OCKZ4DmGYgNiEYFDZ0uxf6lyfJyYsiTxzeukHOjtDe3xIg660aYdWKV4bbog9AmkdXL7x0lTkUb+ERVhMCvtIFE7YKGZqeEovL6tgXl9U/ApdXK/xT0283CWoKBQVcvZUEqimtWTaEFekFD4PTDkwfUg6WZY6Gy6yTU4HESziSh5e0raH7mP4YJ8tZsdtnfIL+NRvReUqFz8goG6Dm0nvsvwcI8jJhH8lGbPxd6hqbvk+PnttZRr5uxiIJwIx/98fW+mAL0N7AScRklFSjQgr4dRTqZ+/TXyUj9E0x/nyaEpRuj83SzLSwFsc2izoxNCSJDz3m5t7RW2Inm3X3oZmkFOdWL4Y1yGIHcFY0i9LSgHYaQpfLpDz4WnlkkU8cyf73Ic= rawda@rawda-Inspiron-3576`

	//os.Getenv("PUBLICKEY")

	nodeID := uint32(3)

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
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
		RootfsSize: 20 * 1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY":  publicKey,
			"TEST_VAR": "this value for test",
		},
		IP:          "10.1.2.5",
		NetworkName: "testingNetwork",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("vm", nodeID, "", nil, "testingNetwork", nil, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.stateLoader.LoadVMFromGrid(nodeID, "vm")
	assert.NoError(t, err)
	assert.Equal(t, v.IP, "10.1.2.5")
	fmt.Printf("v: %v\n", v)

	err = tfPluginClient.DeploymentDeployer.Cancel(ctx, &dl)
	assert.NoError(t, err)

	err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
	assert.NoError(t, err)
}
