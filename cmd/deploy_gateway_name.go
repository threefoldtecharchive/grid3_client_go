// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
)

// deployGatewayNameCmd represents the deploy gateway name command
var deployGatewayNameCmd = &cobra.Command{
	Use:   "name",
	Short: "Deploy a gateway name proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, tls, zosBackends, node, err := parseCommonGatewayFlags(cmd)
		if err != nil {
			return err
		}
		gateway := workloads.GatewayNameProxy{
			Name:           name,
			NodeID:         node,
			Backends:       zosBackends,
			TLSPassthrough: tls,
			SolutionType:   name,
		}
		cfg, err := config.GetUserConfig()
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		t, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		resGateway, err := t.DeployGatewayName(gateway)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msgf("fqdn: %s", resGateway.FQDN)
		return nil
	},
}

func init() {
	deployGatewayCmd.AddCommand(deployGatewayNameCmd)

}
