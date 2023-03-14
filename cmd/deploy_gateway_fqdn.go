// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
	"github.com/threefoldtech/grid3-go/workloads"
)

// deployGatewayFQDNCmd represents the deploy gateway fqdn command
var deployGatewayFQDNCmd = &cobra.Command{
	Use:   "gateway fqdn",
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
		err = command.DeployGatewayFQDN(gateway)
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
