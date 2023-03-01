// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/spf13/cobra"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a deployed resource from Threefold grid",
}

func init() {
	rootCmd.AddCommand(getCmd)
}
