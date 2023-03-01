// Package cmd for parsing command line arguments
package cmd

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/deployer"
)

// cancelCmd represents the cancel command
var cancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel resources on Threefold grid",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msgf("canceling contracts for project %s", args[0])
		// TODO: get mnemonics and network from login command
		mnemonics := os.Getenv("MNEMONICS")
		gridNetwork := os.Getenv("NETWORK")
		tfclient, err := deployer.NewTFPluginClient(mnemonics, "sr25519", gridNetwork, "", "", "", true, false)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		contracts, err := tfclient.ContractsGetter.ListContractsOfProjectName(args[0])

		contractsSlice := append(contracts.NameContracts, contracts.NodeContracts...)
		for _, contract := range contractsSlice {
			contractID, err := strconv.ParseUint(contract.ContractID, 0, 64)
			if err != nil {
				log.Fatal().Err(err).Send()
			}
			log.Debug().Msgf("canceling contract %d", contractID)
			err = tfclient.SubstrateConn.CancelContract(tfclient.Identity, contractID)
			if err != nil {
				log.Fatal().Err(err).Send()
			}
		}
		log.Info().Msgf("%s canceled", args[0])
	},
}

func init() {
	rootCmd.AddCommand(cancelCmd)

}
