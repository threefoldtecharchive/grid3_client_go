// Package deployer for deployer tests
package deployer

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

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

// SSHKeys generates ssh keys in a temp directory and set env PUBLICKEY and PRIVATEKEY with them
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

	publicKey := os.Getenv("PUBLICKEY")

	network := workloads.ZNet{
		Name:        "testingNetwork",
		Description: "network for testing",
		Nodes:       []uint32{14},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	vm := workloads.VM{
		Name:       "vm",
		Flist:      "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-20.04.flist",
		CPU:        2,
		Planetary:  true,
		Memory:     1024,
		RootfsSize: 20 * 1024,
		Entrypoint: "/init.sh",
		EnvVars: map[string]string{
			"SSH_KEY":  publicKey,
			"TEST_VAR": "this value for test",
		},
		IP:          "10.1.0.2",
		NetworkName: "testingNetwork",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)
	assert.NoError(t, err)

	dl := workloads.NewDeployment("vm", 14, "", nil, "testingNetwork", nil, nil, []workloads.VM{vm}, nil)
	err = tfPluginClient.DeploymentDeployer.Deploy(ctx, tfPluginClient.SubstrateConn, &dl)
	assert.NoError(t, err)

	v, err := tfPluginClient.DeploymentDeployer.deployer.stateLoader.LoadVMFromGrid(14, "vm")
	assert.NoError(t, err)
	fmt.Printf("v: %v\n", v)
}
