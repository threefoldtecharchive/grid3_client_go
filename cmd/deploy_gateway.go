// Package cmd for parsing command line arguments
package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// deployGatewayCmd represents the deploy gateway command
var deployGatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Deploy a gateway proxy",
}

func init() {
	deployCmd.AddCommand(deployGatewayCmd)

	deployGatewayCmd.PersistentFlags().StringP("name", "n", "", "name of the gateway")
	err := deployGatewayCmd.MarkPersistentFlagRequired("name")
	if err != nil {
		fmt.Println("hi")
		log.Fatal().Err(err).Send()
	}
	deployGatewayCmd.PersistentFlags().Uint32("node", 0, "node id")
	err = deployGatewayCmd.MarkPersistentFlagRequired("node")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployGatewayCmd.PersistentFlags().StringSlice("backends", []string{}, "backends for the gateway")
	err = deployGatewayCmd.MarkPersistentFlagRequired("backends")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	deployGatewayCmd.PersistentFlags().Bool("tls", false, "add tls passthrough")
}

func parseCommonGatewayFlags(cmd *cobra.Command) (
	name string,
	tls bool,
	zosBackends []zos.Backend,
	node uint32,
	err error,
) {
	name, err = cmd.Flags().GetString("name")
	if err != nil {
		return
	}
	tls, err = cmd.Flags().GetBool("tls")
	if err != nil {
		return
	}
	backends, err := cmd.Flags().GetStringSlice("backends")
	if err != nil {
		return
	}
	for _, backend := range backends {
		zosBackends = append(zosBackends, zos.Backend(backend))
	}
	node, err = cmd.Flags().GetUint32("node")
	if err != nil {
		return
	}
	return
}
