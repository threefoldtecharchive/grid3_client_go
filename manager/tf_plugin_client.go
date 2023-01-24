// Package manager for grid manager
package manager

import (
	"context"
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

// TFPluginClient is Threefold plugin client
type TFPluginClient struct {
	twinID          uint32
	mnemonics       string
	substrateURL    string
	rmbRedisURL     string
	useRmbProxy     bool
	gridProxyClient proxy.Client
	rmb             rmb.Client
	substrateConn   subi.SubstrateExt
	//manager         subi.Manager
	identity substrate.Identity
	manager  DeploymentManager
}

// NewTFPluginClient generates a new tf plugin client
func NewTFPluginClient(ctx context.Context,
	mnemonics string,
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
	tfPluginClient.mnemonics = mnemonics

	var identity substrate.Identity
	switch keyType {
	case "ed25519":
		identity, err = substrate.NewIdentityFromEd25519Phrase(string(tfPluginClient.mnemonics))
	case "sr25519":
		identity, err = substrate.NewIdentityFromSr25519Phrase(string(tfPluginClient.mnemonics))
	default:
		err = errors.New("key type must be one of ed25519 and sr25519")
	}

	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "error getting identity")
	}
	tfPluginClient.identity = identity

	keyPair, err := identity.KeyPair()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "error getting user's identity key pair")
	}

	if network != "dev" && network != "qa" && network != "test" && network != "main" {
		return TFPluginClient{}, errors.New("network must be one of dev, qa, test, and main")
	}

	tfPluginClient.substrateURL = SubstrateURLs[network]
	if len(strings.TrimSpace(substrateURL)) != 0 {
		log.Printf("using a custom substrate url %s", substrateURL)
		if err := validateSubstrateURL(substrateURL); err != nil {
			return TFPluginClient{}, errors.Wrap(err, "couldn't validate substrate url")
		}
		tfPluginClient.substrateURL = substrateURL
	}
	log.Printf("substrate url: %s %s\n", tfPluginClient.substrateURL, substrateURL)

	manager := subi.NewManager(tfPluginClient.substrateURL)
	sub, err := manager.SubstrateExt()
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't get substrate client")
	}

	if err := validateAccount(&tfPluginClient, sub); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate substrate account")
	}
	tfPluginClient.substrateConn = sub

	twinID, err := sub.GetTwinByPubKey(keyPair.Public())
	if err != nil && errors.Is(err, substrate.ErrNotFound) {
		return TFPluginClient{}, errors.Wrap(err, "no twin associated with the account with the given mnemonics")
	}
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "failed to get twin for the given mnemonics")
	}
	tfPluginClient.twinID = twinID

	rmbProxyURL := RMBProxyURLs[network]
	if len(strings.TrimSpace(passedRmbProxyURL)) != 0 {
		if err := validateProxyURL(passedRmbProxyURL); err != nil {
			return TFPluginClient{}, errors.Wrap(err, "couldn't validate rmb proxy url")
		}
		rmbProxyURL = passedRmbProxyURL
	}

	tfPluginClient.useRmbProxy = useRmbProxy
	tfPluginClient.rmbRedisURL = rmbRedisURL

	var rmbClient rmb.Client
	if tfPluginClient.useRmbProxy {
		rmbClient, err = client.NewProxyBus(rmbProxyURL, tfPluginClient.twinID, tfPluginClient.substrateConn, identity, verifyReply)
	} else {
		rmbClient, err = rmb.NewRMBClient(tfPluginClient.rmbRedisURL)
	}
	if err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't create rmb client")
	}
	tfPluginClient.rmb = rmbClient

	gridProxyClient := proxy.NewClient(rmbProxyURL)
	tfPluginClient.gridProxyClient = proxy.NewRetryingClient(gridProxyClient)
	if err := validateClientRMB(&tfPluginClient, sub); err != nil {
		return TFPluginClient{}, errors.Wrap(err, "couldn't validate rmb proxy client")
	}

	return tfPluginClient, nil
}
