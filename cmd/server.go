/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/server"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a redis server that takes commands to manage deployments on Threefold Grid",
	RunE: func(cmd *cobra.Command, args []string) error {
		return server.Serve(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
