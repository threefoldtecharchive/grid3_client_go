// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
)

// deployGatewayFQDNCmd represents the deploy gateway fqdn command
var deployGatewayFQDNCmd = &cobra.Command{
	Use:   "fqdn",
	Short: "Deploy a gateway FQDN proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, tls, zosBackends, node, err := parseCommonGatewayFlags(cmd)
		if err != nil {
			return err
		}
		fqdn, err := cmd.Flags().GetString("fqdn")
		if err != nil {
			return err
		}
		gateway := workloads.GatewayFQDNProxy{
			Name:           name,
			NodeID:         node,
			Backends:       zosBackends,
			TLSPassthrough: tls,
			SolutionType:   name,
			FQDN:           fqdn,
		}
		cfg, err := config.GetUserConfig()
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		t, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		err = t.DeployGatewayFQDN(gateway)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("gateway fqdn deployed")
		return nil
	},
}

func init() {
	deployGatewayCmd.AddCommand(deployGatewayFQDNCmd)

	deployGatewayFQDNCmd.Flags().String("fqdn", "", "fqdn pointing to the specified node")
	err := deployGatewayFQDNCmd.MarkFlagRequired("fqdn")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

}
