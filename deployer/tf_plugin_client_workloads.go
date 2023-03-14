// Package deployer for grid deployer
package deployer

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// DeployVM deploys a vm with mounts
func (t *TFPluginClient) DeployVM(vm workloads.VM, mount workloads.Disk) (workloads.VM, error) {
	node, err := getAvailableNode(t.GridProxyClient, vm, mount.SizeGB)
	if err != nil {
		return workloads.VM{}, err
	}

	networkName := fmt.Sprintf("%snetwork", vm.Name)
	network := buildNetwork(networkName, vm.Name, []uint32{node})

	mounts := []workloads.Disk{}
	if mount.SizeGB != 0 {
		mounts = append(mounts, mount)
	}
	vm.NetworkName = networkName
	dl := workloads.NewDeployment(vm.Name, node, vm.Name, nil, networkName, mounts, nil, []workloads.VM{vm}, nil)

	log.Info().Msg("deploying network")
	err = t.NetworkDeployer.Deploy(context.Background(), &network)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to deploy network on node %d", node)
	}
	log.Info().Msg("deploying vm")
	err = t.DeploymentDeployer.Deploy(context.Background(), &dl)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to deploy vm on node %d", node)
	}
	resVM, err := t.State.LoadVMFromGrid(node, vm.Name, dl.Name)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to load vm from node %d", node)
	}
	return resVM, nil
}

// DeployKubernetesCluster deploys a kubernetes cluster
func (t *TFPluginClient) DeployKubernetesCluster(master workloads.K8sNode, workers []workloads.K8sNode, sshKey string) (workloads.K8sCluster, error) {

	networkName := fmt.Sprintf("%snetwork", master.Name)
	networkNodes := []uint32{master.Node}
	if len(workers) > 0 && workers[0].Node != master.Node {
		networkNodes = append(networkNodes, workers[0].Node)
	}
	network := buildNetwork(networkName, master.Name, networkNodes)

	cluster := workloads.K8sCluster{
		Master:  &master,
		Workers: workers,
		// TODO: should be randomized
		Token:        "securetoken",
		SolutionType: master.Name,
		SSHKey:       sshKey,
		NetworkName:  networkName,
	}
	log.Info().Msg("deploying network")
	err := t.NetworkDeployer.Deploy(context.Background(), &network)
	if err != nil {
		return workloads.K8sCluster{}, errors.Wrapf(err, "failed to deploy network on nodes %v", network.Nodes)
	}
	log.Info().Msg("deploying cluster")
	err = t.K8sDeployer.Deploy(context.Background(), &cluster)
	if err != nil {
		return workloads.K8sCluster{}, errors.Wrap(err, "failed to deploy kubernetes cluster")
	}
	var workersNames []string
	for _, worker := range workers {
		workersNames = append(workersNames, worker.Name)
	}
	workersNodes := make(map[uint32][]string)
	if len(workersNames) > 0 {
		workersNodes[workers[0].Node] = workersNames
	}
	return t.State.LoadK8sFromGrid(
		map[uint32]string{master.Node: master.Name},
		workersNodes,
		master.Name,
	)
}

// DeployGatewayName deploys a gateway name
func (t *TFPluginClient) DeployGatewayName(gateway workloads.GatewayNameProxy) (workloads.GatewayNameProxy, error) {
	log.Info().Msg("deploying gateway name")
	err := t.GatewayNameDeployer.Deploy(context.Background(), &gateway)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "failed to deploy gateway on node %d", gateway.NodeID)
	}
	return t.State.LoadGatewayNameFromGrid(gateway.NodeID, gateway.Name, gateway.Name)
}

// DeployGatewayFQDN deploys a gateway fqdn
func (t *TFPluginClient) DeployGatewayFQDN(gateway workloads.GatewayFQDNProxy) error {

	log.Info().Msg("deploying gateway fqdn")
	err := t.GatewayFQDNDeployer.Deploy(context.Background(), &gateway)
	if err != nil {
		return errors.Wrapf(err, "failed to deploy gateway on node %d", gateway.NodeID)
	}
	return nil
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
	nodes, err := FilterNodes(client, filter)

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

func buildNetwork(name, projectName string, nodes []uint32) workloads.ZNet {
	return workloads.ZNet{
		Name:  name,
		Nodes: nodes,
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 20, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		SolutionType: projectName,
	}
}

// GetVM returns deployed vm
func (t *TFPluginClient) GetVM(name string) (workloads.VM, error) {

	workloadVM, dl, _, err := t.getProjectWorkload(name, "vm")
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to get vm %s", name)
	}
	return workloads.NewVMFromWorkload(&workloadVM, &dl)
}

// GetGatewayName returns deployed gateway name
func (t *TFPluginClient) GetGatewayName(name string) (workloads.GatewayNameProxy, error) {
	workloadGateway, dl, nodeID, err := t.getProjectWorkload(name, "Gateway Name")
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "failed to get gateway name %s", name)
	}
	nameContractID, err := t.SubstrateConn.GetContractIDByNameRegistration(workloadGateway.Name.String())
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "failed to get gateway name contract %s", name)
	}
	gateway, err := workloads.NewGatewayNameProxyFromZosWorkload(workloadGateway)
	if err != nil {
		return workloads.GatewayNameProxy{}, err
	}
	// fields not returned by grid
	gateway.NameContractID = nameContractID
	gateway.ContractID = dl.ContractID
	gateway.NodeID = nodeID
	gateway.SolutionType = name
	gateway.NodeDeploymentID = map[uint32]uint64{nodeID: dl.ContractID}
	return gateway, nil
}

// GetGatewayFQDN returns deployed gateway fqdn
func (t *TFPluginClient) GetGatewayFQDN(name string) (workloads.GatewayFQDNProxy, error) {
	workloadGateway, dl, nodeID, err := t.getProjectWorkload(name, "Gateway Fqdn")
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "failed to get gateway fqdn %s", name)
	}
	gateway, err := workloads.NewGatewayFQDNProxyFromZosWorkload(workloadGateway)
	if err != nil {
		return workloads.GatewayFQDNProxy{}, err
	}
	// fields not returned by grid
	gateway.ContractID = dl.ContractID
	gateway.NodeID = nodeID
	gateway.SolutionType = name
	gateway.NodeDeploymentID = map[uint32]uint64{nodeID: dl.ContractID}
	return workloads.NewGatewayFQDNProxyFromZosWorkload(workloadGateway)
}
