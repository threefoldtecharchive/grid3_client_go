// Package cmd for parsing command line arguments
package cmd

import (
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
		if vm.PublicIP {
			log.Info().Msgf("vm ipv4: %s", vm.ComputedIP)
		}
		if vm.PublicIP6 {
			log.Info().Msgf("vm ipv6: %s", vm.ComputedIP6)
		}
		if vm.Planetary {
			log.Info().Msgf("vm yggdrasil ip: %s", vm.YggIP)
		}

	},
}

func init() {
	getCmd.AddCommand(getVMCmd)
}
