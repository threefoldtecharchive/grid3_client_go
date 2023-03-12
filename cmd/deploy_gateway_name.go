// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// deployGatewayNameCmd represents the deploy gateway-name command
var deployGatewayNameCmd = &cobra.Command{
	Use:   "gateway-name",
	Short: "Deploy a gateway name proxy",
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
		gateway := workloads.GatewayNameProxy{
			Name:           name,
			NodeID:         node,
			Backends:       zosBackends,
			TLSPassthrough: tls,
			SolutionType:   name,
		}
		resGateway, err := command.DeployGatewayName(gateway)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msgf("fqdn: %s", resGateway.FQDN)
		return nil
	},
}

func init() {
	deployCmd.AddCommand(deployGatewayNameCmd)

	deployGatewayNameCmd.Flags().StringP("name", "n", "", "name of the gateway")
	err := deployGatewayNameCmd.MarkFlagRequired("name")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayNameCmd.Flags().Uint32("node", 0, "node id")
	err = deployGatewayNameCmd.MarkFlagRequired("node")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayNameCmd.Flags().StringSlice("backends", []string{}, "backends for the gateway")
	err = deployGatewayNameCmd.MarkFlagRequired("backends")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	deployGatewayNameCmd.Flags().Bool("tls", false, "add tls passthrough")
}
