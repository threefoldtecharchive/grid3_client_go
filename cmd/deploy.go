// Package cmd for parsing command line arguments
package cmd

import (
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy resources to Threefold grid",
}

func init() {
	rootCmd.AddCommand(deployCmd)

}
