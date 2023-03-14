// Package cmd for parsing command line arguments
package cmd

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
)

// getGatewayFQDNCmd represents the get gateway fqdn command
var getGatewayFQDNCmd = &cobra.Command{
	Use:   "fqdn",
	Short: "Get deployed gateway FQDN",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.GetUserConfig()
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		t, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		gateway, err := t.GetGatewayFQDN(args[0])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		s, err := json.MarshalIndent(gateway, "", "\t")
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("gateway fqdn:\n" + string(s))
	},
}

func init() {
	getGatewayCmd.AddCommand(getGatewayFQDNCmd)
}
