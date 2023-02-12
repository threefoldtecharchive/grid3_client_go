// Package deployer for grid deployer
package deployer

import (
	"fmt"
	"io"
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
	twinID       uint32
	mnemonics    string
	identity     substrate.Identity
	substrateURL string
	rmbProxyURL  string
	useRmbProxy  bool

	// network
	Network string

	// clients
	GridProxyClient proxy.Client
	RMB             rmb.Client
	SubstrateConn   subi.SubstrateExt
	NcPool          client.NodeClientGetter

	// deployers
	DeploymentDeployer  DeploymentDeployer
	NetworkDeployer     NetworkDeployer
	GatewayFQDNDeployer GatewayFQDNDeployer
	GatewayNameDeployer GatewayNameDeployer
	K8sDeployer         K8sDeployer

	// state
	StateLoader *StateLoader
}

// NewTFPluginClient generates a new tf plugin client
func NewTFPluginClient(
	mnemonics string,
	keyType string,
	network string,
	substrateURL string,
	passedRmbProxyURL string,
	verifyReply bool,
	showLogs bool,
) (TFPluginClient, error) {

	// disable logging
	if !showLogs {
		log.SetOutput(io.Discard)
	}
	var err error
	tfPluginClient := TFPluginClient{}

	if err := validateMnemonics(mnemonics); err != nil {
		return TFPluginClient{}, errors.Wrapf(err, "couldn't validate mnemonics %s", mnemonics)
	}
	tfPluginClient.mnemonics = mnemonics

	var identity substrate.Identity
	switch keyType {
	case "ed25519":
		identity, err = substrate.NewIdentityFromEd25519Phrase(string(tfPluginClient.mnemonics))
	case "sr25519":
		identity, err = substrate.NewIdentityFromSr25519Phrase(string(tfPluginClient.mnemonics))
	default:
		err = fmt.Errorf("key type must be one of ed25519 and sr25519 not %s", keyType)
	}

	if err != nil {
		return TFPluginClient{}, errors.Wrapf(err, "error getting identity using mnemonics %s", mnemonics)
	}
	tfPluginClient.identity = identity

	keyPair, err := identity.KeyPair()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "error getting user's identity key pair")
	}

	if network != "dev" && network != "qa" && network != "test" && network != "main" {
		return TFPluginClient{}, fmt.Errorf("network must be one of dev, qa, test, and main not %s", network)
	}
	tfPluginClient.Network = network

	tfPluginClient.substrateURL = SubstrateURLs[network]
	if len(strings.TrimSpace(substrateURL)) != 0 {
		if err := validateSubstrateURL(substrateURL); err != nil {
			return TFPluginClient{}, errors.Wrapf(err, "couldn't validate substrate url %s", substrateURL)
		}
		tfPluginClient.substrateURL = substrateURL
	}

	manager := subi.NewManager(tfPluginClient.substrateURL)
	sub, err := manager.SubstrateExt()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't get substrate client")
	}

	if err := validateAccount(sub, tfPluginClient.identity, tfPluginClient.mnemonics); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate substrate account")
	}

	tfPluginClient.SubstrateConn = sub

	twinID, err := sub.GetTwinByPubKey(keyPair.Public())
	if err != nil && errors.Is(err, substrate.ErrNotFound) {
		return TFPluginClient{}, errors.Wrap(err, "no twin associated with the account with the given mnemonics")
	}
	if err != nil {
		return TFPluginClient{}, errors.Wrapf(err, "failed to get twin for the given mnemonics %s", mnemonics)
	}
	tfPluginClient.twinID = twinID

	tfPluginClient.rmbProxyURL = RMBProxyURLs[network]
	if len(strings.TrimSpace(passedRmbProxyURL)) != 0 {
		if err := validateProxyURL(passedRmbProxyURL); err != nil {
			return TFPluginClient{}, errors.Wrapf(err, "couldn't validate rmb proxy url %s", passedRmbProxyURL)
		}
		tfPluginClient.rmbProxyURL = passedRmbProxyURL
	}

	tfPluginClient.useRmbProxy = true
	// if tfPluginClient.useRmbProxy
	rmbClient, err := client.NewProxyBus(tfPluginClient.rmbProxyURL, tfPluginClient.twinID, tfPluginClient.SubstrateConn, identity, verifyReply)
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't create rmb client")
	}
	if err := validateTwinYggdrasil(tfPluginClient.SubstrateConn, tfPluginClient.twinID); err != nil {
		return TFPluginClient{}, errors.Wrapf(err, "couldn't validate twin %d yggdrasil", tfPluginClient.twinID)
	}
	tfPluginClient.RMB = rmbClient

	gridProxyClient := proxy.NewClient(tfPluginClient.rmbProxyURL)
	if err := validateRMBProxyServer(gridProxyClient); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate rmb proxy server")
	}
	tfPluginClient.GridProxyClient = proxy.NewRetryingClient(gridProxyClient)

	ncPool := client.NewNodeClientPool(tfPluginClient.RMB)
	tfPluginClient.NcPool = ncPool

	tfPluginClient.DeploymentDeployer = NewDeploymentDeployer(&tfPluginClient)
	tfPluginClient.NetworkDeployer = NewNetworkDeployer(&tfPluginClient)
	tfPluginClient.GatewayFQDNDeployer = NewGatewayFqdnDeployer(&tfPluginClient)
	tfPluginClient.K8sDeployer = NewK8sDeployer(&tfPluginClient)
	tfPluginClient.GatewayNameDeployer = NewGatewayNameDeployer(&tfPluginClient)

	tfPluginClient.StateLoader = NewStateLoader(tfPluginClient.NcPool, tfPluginClient.SubstrateConn)

	return tfPluginClient, nil
}
