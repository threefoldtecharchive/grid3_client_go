package server

import (
	"context"

	"github.com/threefoldtech/zbus"
)

const ModuleName = "tfgrid-client"
const ServerAddress = "tcp://localhost:6379"
const ServerWorkersNumber = 10

func Serve(ctx context.Context) error {
	server, err := zbus.NewRedisServer(ModuleName, ServerAddress, ServerWorkersNumber)
	if err != nil {
		return err
	}
	impl := tfGridServerImpl{}

	err = server.Register(zbus.ObjectIDFromString("tfgrid-client@v1.0.0"), impl)
	if err != nil {
		return err
	}
	return server.Run(ctx)
}
