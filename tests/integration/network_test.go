package integration

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func TestDeployment(t *testing.T) {

	manager, apiClient := setup()

	network := workloads.TargetNetwork{
		Name:        "net1",
		Description: "not skynet",
		Nodes:       []uint32{33, 34},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: true,
	}
	// vm := workloads.VM{
	// 	Name: "vm1",
	// }

	access, err := network.Stage(context.Background(), apiClient)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)
	defer manager.CancelAll()

	ln, err := loader.LoadNetworkFromGrid(manager, "net1")
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	// network.AddWGAccess = true
	network.Nodes = []uint32{33, 31}
	access, err = network.Stage(context.Background(), apiClient)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.Equal(t, nil, err)

	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	network.AddWGAccess = false
	network.Nodes = []uint32{33, 31}
	access, err = network.Stage(context.Background(), apiClient)
	assert.Equal(t, nil, err)
	log.Printf("user access: %+v", access)

	err = manager.Commit(context.Background())
	assert.NoError(t, err)
	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())

	err = manager.CancelAll()
	ln, err = loader.LoadNetworkFromGrid(manager, "net1")
	log.Printf("current network: %+v", ln)
	log.Printf("current contracts: %+v", manager.GetContractIDs())
}
