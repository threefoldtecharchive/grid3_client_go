// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// deployGatewayFQDNCmd represents the deploy gateway-fqdn command
var deployGatewayFQDNCmd = &cobra.Command{
	Use:   "gateway-fqdn",
	Short: "Deploy a gateway FQDN proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		tls, err := cmd.Flags().GetBool("tls")
		if err != nil {
			return err
		}
		backends, err := cmd.Flags().GetStringSlice("backends")
		if err != nil {
			return err
		}
		zosBackends := []zos.Backend{}
		for _, backend := range backends {
			zosBackends = append(zosBackends, zos.Backend(backend))
		}
		node, err := cmd.Flags().GetUint32("node")
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
	deployCmd.AddCommand(deployGatewayFQDNCmd)

	deployGatewayFQDNCmd.Flags().StringP("name", "n", "", "name of the gateway")
	err := deployGatewayFQDNCmd.MarkFlagRequired("name")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayFQDNCmd.Flags().Uint32("node", 0, "node id")
	err = deployGatewayFQDNCmd.MarkFlagRequired("node")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayFQDNCmd.Flags().StringSlice("backends", []string{}, "backends for the gateway")
	err = deployGatewayFQDNCmd.MarkFlagRequired("backends")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	deployGatewayFQDNCmd.Flags().String("fqdn", "", "fqdn pointing to the specified node")
	err = deployGatewayFQDNCmd.MarkFlagRequired("fqdn")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayFQDNCmd.Flags().Bool("tls", false, "add tls passthrough")

}
