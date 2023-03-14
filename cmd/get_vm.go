// Package cmd for parsing command line arguments
package cmd

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
)

// getVMCmd represents the get vm command
var getVMCmd = &cobra.Command{
	Use:   "vm",
	Short: "Get deployed vm",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		vm, err := command.GetVM(args[0])
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		s, err := json.MarshalIndent(vm, "", "\t")
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("vm:\n" + string(s))

	},
}

func init() {
	getCmd.AddCommand(getVMCmd)
}
