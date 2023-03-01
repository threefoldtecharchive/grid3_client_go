// package cmd for handling commands
package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// DeployVM deploys a vm with mounts
func DeployVM(vm workloads.VM, mount workloads.Disk) (workloads.VM, error) {
	path, err := config.GetConfigPath()
	if err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to get configuration file")
	}

	var cfg config.Config
	err = cfg.Load(path)
	if err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to load configuration try to login again using gridify login")
	}
	tfclient, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return workloads.VM{}, err
	}
	node, err := getAvailableNode(tfclient.GridProxyClient, vm, mount.SizeGB)
	if err != nil {
		return workloads.VM{}, err
	}

	networkName := fmt.Sprintf("%snetwork", vm.Name)
	network := workloads.ZNet{
		Name:  networkName,
		Nodes: []uint32{node},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		SolutionType: vm.Name,
	}

	mounts := []workloads.Disk{}
	if mount.SizeGB != 0 {
		mounts = append(mounts, mount)
	}
	vm.NetworkName = networkName
	dl := workloads.NewDeployment(vm.Name, node, vm.Name, nil, networkName, mounts, nil, []workloads.VM{vm}, nil)

	log.Info().Msg("deploying network")
	err = tfclient.NetworkDeployer.Deploy(context.Background(), &network)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to deploy network on node %d", node)
	}
	log.Info().Msg("deploying vm")
	err = tfclient.DeploymentDeployer.Deploy(context.Background(), &dl)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to deploy vm on node %d", node)
	}
	resVM, err := tfclient.State.LoadVMFromGrid(node, vm.Name, dl.Name)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to load vm from node %d", node)
	}
	return resVM, nil
}

func getAvailableNode(client client.Client, vm workloads.VM, diskSize int) (uint32, error) {
	nodeStatus := "up"
	freeMRU := uint64(vm.Memory / 1024)
	freeHRU := uint64(vm.RootfsSize/1024 + diskSize)
	freeIPs := uint64(0)
	domain := true
	if vm.PublicIP {
		freeIPs = 1
	}
	filter := types.NodeFilter{
		FarmIDs: []uint64{1},
		Status:  &nodeStatus,
		FreeMRU: &freeMRU,
		FreeHRU: &freeHRU,
		FreeIPs: &freeIPs,
		Domain:  &domain,
	}
	nodes, err := deployer.FilterNodes(client, filter)

	if err != nil {
		return 0, err
	}
	if len(nodes) == 0 {
		return 0, fmt.Errorf(
			"no node with free resources available using node filter: farmIDs: %v, mru: %d, hru: %d, freeips: %d, domain: %t",
			filter.FarmIDs,
			*filter.FreeMRU,
			*filter.FreeHRU,
			*filter.FreeIPs,
			*filter.Domain,
		)
	}

	node := uint32(nodes[0].NodeID)
	return node, nil
}
