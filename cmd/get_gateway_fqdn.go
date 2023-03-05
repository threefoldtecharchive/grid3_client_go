// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
)

// getGatewayFQDNCmd represents the get gateway-fqdn command
var getGatewayFQDNCmd = &cobra.Command{
	Use:   "gateway-fqdn",
	Short: "Get deployed gateway FQDN",
	Run: func(cmd *cobra.Command, args []string) {
		gateway, err := command.GetGatewayFQDN(args[0])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msgf("fqdn: %s", gateway.FQDN)
	},
}

func init() {
	getCmd.AddCommand(getGatewayFQDNCmd)
}
