// Package deployer for grid deployer
package deployer

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// DeployMachines deploys a set of vms with disks on the same network on a single node
func (t *TFPluginClient) DeployMachines(
	projectName string,
	vms []workloads.VM,
	mounts []workloads.Disk,
	network workloads.ZNet,
) (resVMs []workloads.VM, WGConfig string, err error) {
	filter := constructNodeFilter(vms, mounts)
	nodes, err := FilterNodes(t.GridProxyClient, filter)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to filter nodes")
	}
	if len(nodes) == 0 {
		return nil, "", fmt.Errorf(
			"no node with free resources available using node filter: cru: %d, sru: %d, mru: %d, hru: %d, freeips: %d",
			*filter.TotalCRU,
			*filter.FreeSRU,
			*filter.FreeMRU,
			*filter.FreeHRU,
			*filter.FreeIPs,
		)
	}
	node := uint32(nodes[0].NodeID)
	network.Nodes = []uint32{node}
	network.SolutionType = projectName

	dl := workloads.NewDeployment(projectName, node, projectName, nil, network.Name, mounts, nil, vms, nil)
	log.Info().Msg("deploying network")
	err = t.NetworkDeployer.Deploy(context.Background(), &network)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to deploy network on node %d", node)
	}
	log.Info().Msg("deploying vm")
	err = t.DeploymentDeployer.Deploy(context.Background(), &dl)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to deploy vm on node %d", node)
	}

	for _, vm := range vms {
		resVM, err := t.State.LoadVMFromGrid(node, vm.Name, dl.Name)
		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to load vm from node %d", node)
		}
		resVMs = append(resVMs, resVM)
	}

	return resVMs, network.AccessWGConfig, nil
}

func constructNodeFilter(vms []workloads.VM, disks []workloads.Disk) types.NodeFilter {
	cru := uint64(0)
	mru := uint64(0)
	sru := uint64(0)
	hru := uint64(0)
	publicIPs := uint64(0)
	nodeStatus := "up"
	for _, vm := range vms {
		if vm.CPU > int(cru) {
			cru = uint64(vm.CPU)
		}
		mru += uint64(vm.Memory / 1024)
		sru += uint64(vm.RootfsSize / 1024)
		if vm.PublicIP {
			publicIPs++
		}
	}
	for _, disk := range disks {
		hru += uint64(disk.SizeGB / 1024)
	}
	return types.NodeFilter{
		Status:   &nodeStatus,
		FreeMRU:  &mru,
		FreeHRU:  &hru,
		FreeSRU:  &sru,
		TotalCRU: &cru,
		FreeIPs:  &publicIPs,
	}
}

// DeployVM deploys a vm with mounts
func (t *TFPluginClient) DeployVM(vm workloads.VM, mount workloads.Disk, node uint32) (workloads.VM, error) {
	networkName := fmt.Sprintf("%snetwork", vm.Name)
	network := buildNetwork(networkName, vm.Name, []uint32{node})

	mounts := []workloads.Disk{}
	if mount.SizeGB != 0 {
		mounts = append(mounts, mount)
	}
	vm.NetworkName = networkName
	dl := workloads.NewDeployment(vm.Name, node, vm.Name, nil, networkName, mounts, nil, []workloads.VM{vm}, nil)

	log.Info().Msg("deploying network")
	err := t.NetworkDeployer.Deploy(context.Background(), &network)
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
	return gateway, nil
}
