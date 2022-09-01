package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/goombaio/namegenerator"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"golang.org/x/crypto/ssh"
)

type APIClient struct {
	SubstrateExt subi.SubstrateExt
	NCPool       *client.NodeClientPool
	ProxyClient  proxy.Client
	Manager      deployer.DeploymentManager
	Identity     subi.Identity
}

var (
	SUBSTRATE_URL = map[string]string{
		"dev":  "wss://tfchain.dev.grid.tf/ws",
		"test": "wss://tfchain.test.grid.tf/ws",
		"qa":   "wss://tfchain.qa.grid.tf/ws",
		"main": "wss://tfchain.grid.tf/ws",
	}
	RMB_PROXY_URL = map[string]string{
		"dev":  "https://gridproxy.dev.grid.tf/",
		"test": "https://gridproxy.test.grid.tf/",
		"qa":   "https://gridproxy.qa.grid.tf/",
		"main": "https://gridproxy.grid.tf/",
	}
)

func setup() (deployer.DeploymentManager, APIClient) {
	mnemonics := os.Getenv("MNEMONICS")
	SshKeys()
	identity, err := subi.NewIdentityFromSr25519Phrase(mnemonics)
	if err != nil {
		panic(err)
	}
	sk, err := identity.KeyPair()
	if err != nil {
		panic(err)
	}
	network := os.Getenv("NETWORK")

	sub := subi.NewManager(SUBSTRATE_URL[network])
	pub := sk.Public()
	subext, err := sub.SubstrateExt()
	if err != nil {
		panic(err)
	}

	defer subext.Close()
	twin, err := subext.GetTwinByPubKey(pub)
	if err != nil {
		panic(err)
	}
	cl, err := client.NewProxyBus(RMB_PROXY_URL[network], twin, sub, identity, true)
	if err != nil {
		panic(err)
	}
	gridClient := proxy.NewRetryingClient(proxy.NewClient(RMB_PROXY_URL[network]))
	ncPool := client.NewNodeClientPool(cl)
	manager := deployer.NewDeploymentManager(identity, twin, map[uint32]uint64{}, gridClient, ncPool, sub)
	apiClient := APIClient{
		SubstrateExt: subext,
		NCPool:       ncPool,
		ProxyClient:  gridClient,
		Manager:      manager,
		Identity:     identity,
	}
	return manager, apiClient
}

// UpWg used for up wireguard
func UpWg(wgConfig string) {
	f, err := os.Create("/tmp/test.conf")

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	_, err2 := f.WriteString(wgConfig)

	if err2 != nil {
		log.Fatal(err2)
	}

	cmd := exec.Command("sudo", "wg-quick", "up", "/tmp/test.conf")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err)
		return
	}

	// Print the output
	fmt.Println(string(stdout))
}

// DownWG used for down wireguard
func DownWG() {
	cmd := exec.Command("sudo", "wg-quick", "down", "/tmp/test.conf")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(stdout))
}

// RemoteRun used for ssh host
func RemoteRun(user string, addr string, cmd string) (string, error) {
	privateKey := os.Getenv("PRIVATEKEY")
	key, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", err
	}
	// Authentication
	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	// Connect
	client, err := ssh.Dial("tcp", net.JoinHostPort(addr, "22"), config)
	if err != nil {
		return "", err
	}
	// Create a session. It is one session per command.
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var b bytes.Buffer  // import "bytes"
	session.Stdout = &b // get output
	err = session.Run(cmd)
	return b.String(), err
}

func VerifyIPs(wgConfig string, verifyIPs []string) bool {
	UpWg(wgConfig)

	for i := 0; i < len(verifyIPs); i++ {
		out, _ := exec.Command("ping", verifyIPs[i], "-c 5", "-i 3", "-w 10").Output()
		if strings.Contains(string(out), "Destination Host Unreachable") {
			return false
		}
	}

	for i := 0; i < len(verifyIPs); i++ {
		res, _ := RemoteRun("root", verifyIPs[i], "ifconfig")
		if !strings.Contains(string(res), verifyIPs[i]) {
			return false
		}
	}
	return true
}

func RandomName() string {
	seed := time.Now().UTC().UnixNano()
	nameGenerator := namegenerator.NewNameGenerator(seed)

	name := nameGenerator.Generate()

	return name
}

func Wait(addr string, port string) bool {
	for t := time.Now(); time.Since(t) < 3*time.Minute; {
		_, err := net.DialTimeout("tcp", net.JoinHostPort(addr, "22"), time.Second*12)
		if err == nil {
			return true
		}
	}
	return true
}

func SshKeys() {
	os.Mkdir("/tmp/.ssh", 0755)
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-f", "/tmp/.ssh/id_rsa", "-q")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(stdout))

	private_key, err := ioutil.ReadFile("/tmp/.ssh/id_rsa")
	if err != nil {
		log.Fatal(err)
	}

	public_key, e := ioutil.ReadFile("/tmp/.ssh/id_rsa.pub")
	if e != nil {
		log.Fatal(err)
	}

	os.Setenv("PUBLICKEY", string(public_key))
	os.Setenv("PRIVATEKEY", string(private_key))
}
