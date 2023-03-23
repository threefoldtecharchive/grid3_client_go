/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/server"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a redis server that takes commands to manage deployments on Threefold Grid",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := &daemon.Context{
			LogFilePerm: 0640,
			WorkDir:     "/",
			Umask:       027,
			LogFileName: "/home/superluigi/gridclient.log",
		}

		child, err := ctx.Reborn()
		if err != nil {
			return errors.Wrap(err, "Unable to run grid client server")
		}

		if child != nil {
			return nil
		}
		defer func() {
			_ = ctx.Release()
		}()

		cl, err := server.NewRedisClient()
		if err != nil {
			return errors.Wrap(err, "failed to create new redis client")
		}
		cl.Listen(context.Background())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
