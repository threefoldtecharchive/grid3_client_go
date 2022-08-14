package main

// import (
// 	"context"
// 	"time"

// 	"github.com/threefoldtech/grid3-go/deployer"
// 	client "github.com/threefoldtech/grid3-go/node"
// 	substratemanager "github.com/threefoldtech/grid3-go/substrate_manager"
// 	"github.com/threefoldtech/grid3-go/workloads"
// 	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
// 	"github.com/threefoldtech/substrate-client"
// )

// var mnemonics string

// func main() {
// 	identity, err := substrate.NewIdentityFromSr25519Phrase(mnemonics)
// 	if err != nil {
// 		panic(err)
// 	}
// 	subManager := substratemanager.NewManager("wss://tfchain.dev.grid.tf/ws")
// 	cl, err := client.NewProxyBus("https://gridproxy.dev.grid.tf/", 192, subManager, identity, true)
// 	manager := deployer.NewDeploymentManager(identity, 192, map[uint32]uint64{}, proxy.NewClient("https://gridproxy.dev.grid.tf/"), client.NewNodeClientPool(cl), subManager)
// 	z := workloads.ZDB{
// 		Name:        "test",
// 		Password:    "password",
// 		Public:      false,
// 		Size:        20,
// 		Description: "test zdb",
// 		Mode:        "user",
// 	}
// 	wl := z.Convert()
// 	manager.SetWorkload(13, wl)
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
// 	defer cancel()
// 	err = manager.Commit(ctx)
// 	if err != nil {
// 		panic(err)
// 	}
// }
