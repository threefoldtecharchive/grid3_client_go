// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
)

// getGatewayNameCmd represents the get gateway-name command
var getGatewayNameCmd = &cobra.Command{
	Use:   "gateway-name",
	Short: "Get deployed gateway name",
	Run: func(cmd *cobra.Command, args []string) {
		gateway, err := command.GetGatewayName(args[0])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msgf("fqdn: %s", gateway.FQDN)
	},
}

func init() {
	getCmd.AddCommand(getGatewayNameCmd)
}
