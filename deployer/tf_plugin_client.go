// Package deployer for grid deployer
package deployer

import (
	"log"
	"strings"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/rmb"
)

var (
	// SubstrateURLs are substrate urls
	SubstrateURLs = map[string]string{
		"dev":  "wss://tfchain.dev.grid.tf/ws",
		"test": "wss://tfchain.test.grid.tf/ws",
		"qa":   "wss://tfchain.qa.grid.tf/ws",
		"main": "wss://tfchain.grid.tf/ws",
	}
	// RMBProxyURLs are rmb proxy urls
	RMBProxyURLs = map[string]string{
		"dev":  "https://gridproxy.dev.grid.tf/",
		"test": "https://gridproxy.test.grid.tf/",
		"qa":   "https://gridproxy.qa.grid.tf/",
		"main": "https://gridproxy.grid.tf/",
	}
)

// TFPluginClient is a Threefold plugin client
type TFPluginClient struct {
	Network string

	TwinID          uint32
	Mnemonics       string
	SubstrateURL    string
	RMBRedisURL     string
	UseRmbProxy     bool
	GridProxyClient proxy.Client
	RMB             rmb.Client
	SubstrateConn   subi.SubstrateExt
	NcPool          client.NodeClientGetter
	Identity        substrate.Identity

	DeploymentDeployer  DeploymentDeployer
	NetworkDeployer     NetworkDeployer
	GatewayFQDNDeployer GatewayFQDNDeployer
	//gatewayNameDeployer GatewayNameDeployer
	//k8sDeployer k8sDeployer

	StateLoader *StateLoader
}

// NewTFPluginClient generates a new tf plugin client
func NewTFPluginClient(mnemonics string,
	keyType string,
	network string,
	substrateURL string,
	passedRmbProxyURL string,
	useRmbProxy bool,
	rmbRedisURL string,
	verifyReply bool,
) (TFPluginClient, error) {

	var err error
	tfPluginClient := TFPluginClient{}

	if err := validateMnemonics(mnemonics); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate mnemonics")
	}
	tfPluginClient.Mnemonics = mnemonics

	var identity substrate.Identity
	switch keyType {
	case "ed25519":
		identity, err = substrate.NewIdentityFromEd25519Phrase(string(tfPluginClient.Mnemonics))
	case "sr25519":
		identity, err = substrate.NewIdentityFromSr25519Phrase(string(tfPluginClient.Mnemonics))
	default:
		err = errors.New("key type must be one of ed25519 and sr25519")
	}

	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "error getting identity")
	}
	tfPluginClient.Identity = identity

	keyPair, err := identity.KeyPair()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "error getting user's identity key pair")
	}

	if network != "dev" && network != "qa" && network != "test" && network != "main" {
		return TFPluginClient{}, errors.New("network must be one of dev, qa, test, and main")
	}
	tfPluginClient.Network = network

	tfPluginClient.SubstrateURL = SubstrateURLs[network]
	if len(strings.TrimSpace(substrateURL)) != 0 {
		log.Printf("using a custom substrate url %s", substrateURL)
		if err := validateSubstrateURL(substrateURL); err != nil {
			return TFPluginClient{}, errors.Wrap(err, "couldn't validate substrate url")
		}
		tfPluginClient.SubstrateURL = substrateURL
	}
	log.Printf("substrate url: %s %s\n", tfPluginClient.SubstrateURL, substrateURL)

	manager := subi.NewManager(tfPluginClient.SubstrateURL)
	sub, err := manager.SubstrateExt()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't get substrate client")
	}

	if err := validateAccount(&tfPluginClient, sub); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate substrate account")
	}

	tfPluginClient.SubstrateConn = sub

	twinID, err := sub.GetTwinByPubKey(keyPair.Public())
	if err != nil && errors.Is(err, substrate.ErrNotFound) {
		return TFPluginClient{}, errors.Wrap(err, "no twin associated with the account with the given mnemonics")
	}
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "failed to get twin for the given mnemonics")
	}
	tfPluginClient.TwinID = twinID

	rmbProxyURL := RMBProxyURLs[network]
	if len(strings.TrimSpace(passedRmbProxyURL)) != 0 {
		if err := validateProxyURL(passedRmbProxyURL); err != nil {
			return TFPluginClient{}, errors.Wrap(err, "couldn't validate rmb proxy url")
		}
		rmbProxyURL = passedRmbProxyURL
	}

	tfPluginClient.UseRmbProxy = useRmbProxy
	tfPluginClient.RMBRedisURL = rmbRedisURL

	var rmbClient rmb.Client
	if tfPluginClient.UseRmbProxy {
		rmbClient, err = client.NewProxyBus(rmbProxyURL, tfPluginClient.TwinID, tfPluginClient.SubstrateConn, identity, verifyReply)
	} else {
		rmbClient, err = rmb.NewRMBClient(tfPluginClient.RMBRedisURL)
	}
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't create rmb client")
	}
	tfPluginClient.RMB = rmbClient

	gridProxyClient := proxy.NewClient(rmbProxyURL)
	tfPluginClient.GridProxyClient = proxy.NewRetryingClient(gridProxyClient)
	if err := validateClientRMB(&tfPluginClient, sub); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate rmb proxy client")
	}

	ncPool := client.NewNodeClientPool(tfPluginClient.RMB)
	tfPluginClient.NcPool = ncPool

	tfPluginClient.DeploymentDeployer = NewDeploymentDeployer(&tfPluginClient)
	tfPluginClient.NetworkDeployer = NewNetworkDeployer(&tfPluginClient)
	tfPluginClient.GatewayFQDNDeployer = NewGatewayFqdnDeployer(&tfPluginClient)

	tfPluginClient.StateLoader = NewStateLoader(tfPluginClient.NcPool, tfPluginClient.SubstrateConn)
	return tfPluginClient, nil
}
