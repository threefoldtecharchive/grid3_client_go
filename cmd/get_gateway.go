// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/spf13/cobra"
)

// getGatewayCmd represents the get gateway command
var getGatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Get deployed gateway",
}

func init() {
	getCmd.AddCommand(getGatewayCmd)
}
