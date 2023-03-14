// Package cmd for parsing command line arguments
package cmd

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
)

// getGatewayNameCmd represents the get gateway name command
var getGatewayNameCmd = &cobra.Command{
	Use:   "name",
	Short: "Get deployed gateway name",
	Run: func(cmd *cobra.Command, args []string) {
		gateway, err := command.GetGatewayName(args[0])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		s, err := json.MarshalIndent(gateway, "", "\t")
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("gateway name:\n" + string(s))
	},
}

func init() {
	getGatewayCmd.AddCommand(getGatewayNameCmd)
}
