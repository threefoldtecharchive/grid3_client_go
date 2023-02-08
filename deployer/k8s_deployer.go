package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"regexp"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// K8s Deployer for deploying k8s
type K8sDeployer struct {
	NodeUsedIPs    map[uint32][]byte
	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// Generate new K8s Deployer
func NewK8sDeployer(tfPluginClient *TFPluginClient) K8sDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	k8sDeployer := K8sDeployer{
		NodeUsedIPs:    map[uint32][]byte{},
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}

	return k8sDeployer
}

// validateToken validates the Token of k8s cluster
func (k *K8sDeployer) validateToken(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if k8sCluster.Token == "" {
		return errors.New("empty token is not allowed")
	}

	is_alphanumeric := regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(k8sCluster.Token)
	if !is_alphanumeric {
		return errors.New("token should be alphanumeric")
	}

	return nil
}

// validateNames validates unique names of masters && workers of k8s cluster
func (k *K8sDeployer) ValidateNames(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	names := make(map[string]bool)
	names[k8sCluster.Master.Name] = true
	for _, w := range k8sCluster.Workers {
		if _, ok := names[w.Name]; ok {
			return fmt.Errorf("k8s workers and master must have unique names: %s occurred more than once", w.Name)
		}
		names[w.Name] = true
	}
	return nil
}

// validateIPranges validates NodesIPRange of master && workers of k8s cluster
func (k *K8sDeployer) ValidateIPranges(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if _, ok := k8sCluster.NodesIPRange[k8sCluster.Master.Node]; !ok {
		return fmt.Errorf("the master node %d doesn't exist in the network's ip ranges", k8sCluster.Master.Node)
	}
	for _, w := range k8sCluster.Workers {
		if _, ok := k8sCluster.NodesIPRange[w.Node]; !ok {
			return fmt.Errorf("the node with id %d in worker %s doesn't exist in the network's ip ranges", w.Node, w.Name)
		}
	}
	return nil
}

// Validate validates K8s deployer
func (k *K8sDeployer) Validate(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	sub := k.tfPluginClient.SubstrateConn
	if err := k.validateToken(ctx, k8sCluster); err != nil {
		return err
	}

	if err := validateAccountBalanceForExtrinsics(sub, k.tfPluginClient.Identity); err != nil {
		return err
	}
	if err := k.ValidateNames(ctx, k8sCluster); err != nil {
		return err
	}
	if err := k.ValidateIPranges(ctx, k8sCluster); err != nil {
		return err
	}
	nodes := make([]uint32, 0)
	nodes = append(nodes, k8sCluster.Master.Node)
	for _, worker := range k8sCluster.Workers {
		if !workloads.Contains(nodes,worker.Node){
			nodes = append(nodes, worker.Node)
		}
	}
	return client.AreNodesUp(ctx, sub, nodes, k.tfPluginClient.NcPool)
}

func (k *K8sDeployer) removeDeletedContracts(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	sub := k.tfPluginClient.SubstrateConn
	nodeDeploymentID := make(map[uint32]uint64)
	for nodeID, deploymentID := range k8sCluster.NodeDeploymentID {
		cont, err := sub.GetContract(deploymentID)
		if err != nil {
			return errors.Wrap(err, "failed to get deployments")
		}
		if !cont.IsDeleted() {
			nodeDeploymentID[nodeID] = deploymentID
		}
	}
	k8sCluster.NodeDeploymentID = nodeDeploymentID
	return nil
}

func (k *K8sDeployer) UpdateFromRemote(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if err := k.removeDeletedContracts(ctx, k8sCluster); err != nil {
		return errors.Wrap(err, "failed to remove deleted contracts")
	}
	currentDeployments, err := k.deployer.GetDeployments(ctx, k8sCluster.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "failed to fetch remote deployments")
	}
	log.Printf("calling updateFromRemote")
	err = PrintDeployments(currentDeployments)
	if err != nil {
		return errors.Wrap(err, "couldn't print deployments data")
	}

	keyUpdated, tokenUpdated, networkUpdated := false, false, false
	// calculate k's properties from the currently deployed deployments
	for _, dl := range currentDeployments {
		for _, w := range dl.Workloads {
			if w.Type == zos.ZMachineType {
				d, err := w.WorkloadData()
				if err != nil {
					log.Printf("failed to get workload data %s", err)
				}
				SSHKey := d.(*zos.ZMachine).Env["SSH_KEY"]
				token := d.(*zos.ZMachine).Env["K3S_TOKEN"]
				networkName := string(d.(*zos.ZMachine).Network.Interfaces[0].Network)
				if !keyUpdated && SSHKey != k8sCluster.SSHKey {
					k8sCluster.SSHKey = SSHKey
					keyUpdated = true
				}
				if !tokenUpdated && token != k8sCluster.Token {
					k8sCluster.Token = token
					tokenUpdated = true
				}
				if !networkUpdated && networkName != k8sCluster.NetworkName {
					k8sCluster.NetworkName = networkName
					networkUpdated = true
				}
			}
		}
	}

	nodeDeploymentID := make(map[uint32]uint64)
	for node, dl := range currentDeployments {
		nodeDeploymentID[node] = dl.ContractID
	}
	k8sCluster.NodeDeploymentID = nodeDeploymentID
	// maps from workload name to (public ip, node id, disk size, actual workload)
	workloadNodeID := make(map[string]uint32)
	workloadDiskSize := make(map[string]int)
	workloadComputedIP := make(map[string]string)
	workloadComputedIP6 := make(map[string]string)
	workloadObj := make(map[string]gridtypes.Workload)

	publicIPs := make(map[string]string)
	publicIP6s := make(map[string]string)
	diskSize := make(map[string]int)
	for node, dl := range currentDeployments {
		for _, w := range dl.Workloads {
			if w.Type == zos.ZMachineType {
				workloadNodeID[string(w.Name)] = node
				workloadObj[string(w.Name)] = w

			} else if w.Type == zos.PublicIPType {
				d := zos.PublicIPResult{}
				if err := json.Unmarshal(w.Result.Data, &d); err != nil {
					log.Printf("failed to load pubip data %s", err)
					continue
				}
				publicIPs[string(w.Name)] = d.IP.String()
				publicIP6s[string(w.Name)] = d.IPv6.String()
			} else if w.Type == zos.ZMountType {
				d, err := w.WorkloadData()
				if err != nil {
					log.Printf("failed to load disk data %s", err)
					continue
				}
				diskSize[string(w.Name)] = int(d.(*zos.ZMount).Size / gridtypes.Gigabyte)
			}
		}
	}
	for _, dl := range currentDeployments {
		for _, w := range dl.Workloads {
			if w.Type == zos.ZMachineType {
				publicIPKey := fmt.Sprintf("%sip", w.Name)
				diskKey := fmt.Sprintf("%sdisk", w.Name)
				workloadDiskSize[string(w.Name)] = diskSize[diskKey]
				workloadComputedIP[string(w.Name)] = publicIPs[publicIPKey]
				workloadComputedIP6[string(w.Name)] = publicIP6s[publicIPKey]
			}
		}
	}
	// update master
	masterNodeID, ok := workloadNodeID[k8sCluster.Master.Name]
	if !ok {
		k8sCluster.Master = nil
	} else {
		masterWorkload := workloadObj[k8sCluster.Master.Name]
		masterIP := workloadComputedIP[k8sCluster.Master.Name]
		masterIP6 := workloadComputedIP6[k8sCluster.Master.Name]
		masterDiskSize := workloadDiskSize[k8sCluster.Master.Name]

		m, err := workloads.NewK8sNodeDataFromWorkload(masterWorkload, masterNodeID, masterDiskSize, masterIP, masterIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get master data from workload")
		}
		k8sCluster.Master = &m
	}
	// update workers
	workers := make([]workloads.K8sNodeData, 0)
	for _, w := range k8sCluster.Workers {
		workerNodeID, ok := workloadNodeID[w.Name]
		if !ok {
			// worker doesn't exist in any deployment, skip it
			continue
		}
		delete(workloadNodeID, w.Name)
		workerWorkload := workloadObj[w.Name]
		workerIP := workloadComputedIP[w.Name]
		workerIP6 := workloadComputedIP6[w.Name]

		workerDiskSize := workloadDiskSize[w.Name]
		w, err := workloads.NewK8sNodeDataFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get worker data from workload")
		}
		workers = append(workers, w)
	}
	// add missing workers (in case of failed deletions)
	for name, workerNodeID := range workloadNodeID {
		if name == k8sCluster.Master.Name {
			continue
		}
		workerWorkload := workloadObj[name]
		workerIP := workloadComputedIP[name]
		workerIP6 := workloadComputedIP6[name]
		workerDiskSize := workloadDiskSize[name]
		w, err := workloads.NewK8sNodeDataFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get worker data from workload")
		}
		workers = append(workers, w)
	}
	k8sCluster.Workers = workers
	log.Printf("after updateFromRemote\n")
	enc := json.NewEncoder(log.Writer())
	enc.SetIndent("", "  ")
	err = enc.Encode(k)
	if err != nil {
		return errors.Wrap(err, "failed to encode k8s deployer")
	}

	return nil
}

func (k *K8sDeployer) validateChecksums(k8sCluster *workloads.K8sCluster) error {
	nodes := append(k8sCluster.Workers, *k8sCluster.Master)
	for _, vm := range nodes {
		if vm.FlistChecksum == "" {
			continue
		}
		checksum, err := workloads.GetFlistChecksum(vm.Flist)
		if err != nil {
			return errors.Wrapf(err, "couldn't get flist %s hash", vm.Flist)
		}
		if vm.FlistChecksum != checksum {
			return fmt.Errorf("passed checksum %s of %s doesn't match %s returned from %s",
				vm.FlistChecksum,
				vm.Name,
				checksum,
				workloads.FlistChecksumURL(vm.Flist),
			)
		}
	}
	return nil
}

func (k *K8sDeployer) getK8sFreeIP(ipRange gridtypes.IPNet, nodeID uint32) (string, error) {
	ip := ipRange.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("the provided ip range (%s) is not a valid ipv4", ipRange.String())
	}

	for i := 2; i < 255; i++ {
		hostID := byte(i)
		if !workloads.Contains(k.NodeUsedIPs[nodeID], hostID) {
			k.NodeUsedIPs[nodeID] = append(k.NodeUsedIPs[nodeID], hostID)
			ip[3] = hostID
			return ip.String(), nil
		}
	}
	return "", errors.New("all ips are used")
}

func (k *K8sDeployer) assignNodesIPs(k8sCluster *workloads.K8sCluster) error {
	masterNodeRange := k8sCluster.NodesIPRange[k8sCluster.Master.Node]
	if k8sCluster.Master.IP == "" || !masterNodeRange.Contains(net.ParseIP(k8sCluster.Master.IP)) {
		ip, err := k.getK8sFreeIP(masterNodeRange, k8sCluster.Master.Node)
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for master")
		}
		k8sCluster.Master.IP = ip
	}
	for idx, w := range k8sCluster.Workers {
		workerNodeRange := k8sCluster.NodesIPRange[w.Node]
		if w.IP != "" && workerNodeRange.Contains(net.ParseIP(w.IP)) {
			continue
		}
		ip, err := k.getK8sFreeIP(workerNodeRange, w.Node)
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for worker")
		}
		k8sCluster.Workers[idx].IP = ip
	}
	return nil
}

func (k *K8sDeployer) GenerateVersionlessDeployments(ctx context.Context, k8sCluster *workloads.K8sCluster) (map[uint32]gridtypes.Deployment, error) {
	err := k.assignNodesIPs(k8sCluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	deployments := make(map[uint32]gridtypes.Deployment)
	nodeWorkloads := make(map[uint32][]gridtypes.Workload)

	masterWorkloads := k8sCluster.Master.GenerateK8sWorkload(k8sCluster, false)
	nodeWorkloads[k8sCluster.Master.Node] = append(nodeWorkloads[k8sCluster.Master.Node], masterWorkloads...)
	for _, w := range k8sCluster.Workers {
		workerWorkloads := w.GenerateK8sWorkload(k8sCluster, true)
		nodeWorkloads[w.Node] = append(nodeWorkloads[w.Node], workerWorkloads...)
	}

	for node, ws := range nodeWorkloads {
		dl := gridtypes.Deployment{
			Version:   0,
			TwinID:    uint32(k.tfPluginClient.TwinID),
			Workloads: ws,
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: k.tfPluginClient.TwinID,
						Weight: 1,
					},
				},
			},
		}
		deployments[node] = dl
	}
	return deployments, nil
}

func (k *K8sDeployer) AssignNodeIpRange(k8sCluster *workloads.K8sCluster) (err error) {
	network := k.tfPluginClient.StateLoader.networks.getNetwork(k8sCluster.NetworkName)
	nodesIPRange := make(map[uint32]gridtypes.IPNet)
	nodesIPRange[k8sCluster.Master.Node], err = gridtypes.ParseIPNet(network.getNodeSubnet(k8sCluster.Master.Node))
	if err != nil {
		return errors.Wrap(err, "couldn't parse master node ip range")
	}
	for _, worker := range k8sCluster.Workers {
		nodesIPRange[worker.Node], err = gridtypes.ParseIPNet(network.getNodeSubnet(worker.Node))
		if err != nil {
			return errors.Wrapf(err, "couldn't parse worker node (%d) ip range", worker.Node)
		}
	}
	k8sCluster.NodesIPRange = nodesIPRange

	return nil
}

func (k *K8sDeployer) Deploy(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if err := k.AssignNodeIpRange(k8sCluster); err != nil {
		return err
	}

	if len(k8sCluster.NodeDeploymentID) != 0 {
		err := k.InvalidateBrokenAttributes(k8sCluster)
		if err != nil {
			return err
		}
	}

	if err := k.Validate(ctx, k8sCluster); err != nil {
		return err
	}
	if err := k.validateChecksums(k8sCluster); err != nil {
		return err
	}
	newDeployments, err := k.GenerateVersionlessDeployments(ctx, k8sCluster)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}
	deploymentData := workloads.DeploymentData{
		Name:        k8sCluster.Master.Name,
		Type:        "K8s",
		ProjectName: "",
	}
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	newDeploymentsData[k8sCluster.Master.Node] = deploymentData
	fmt.Printf("deploymentData: %v\n", deploymentData)
	newDeploymentsSolutionProvider[k8sCluster.Master.Node] = nil

	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment
	fmt.Printf("oldDeployments: %v\n", oldDeployments)

	k8sCluster.NodeDeploymentID, err = k.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)
	fmt.Printf("k8sCluster.NodeDeploymentID: %v\n", k8sCluster.NodeDeploymentID)
	if k8sCluster.ContractID == 0 && k8sCluster.NodeDeploymentID[k8sCluster.Master.Node] != 0 {
		k8sCluster.ContractID = k8sCluster.NodeDeploymentID[k8sCluster.Master.Node]
	}

	k.tfPluginClient.StateLoader.currentNodeDeployment[k8sCluster.Master.Node] = k8sCluster.ContractID
	return err
}

func (k *K8sDeployer) Cancel(ctx context.Context, k8sCluster *workloads.K8sCluster) (err error) {
	if err := k.Validate(ctx, k8sCluster); err != nil {
		return err
	}
	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment
	newDeployments := make(map[uint32]gridtypes.Deployment)
	for nodeID := range oldDeployments {
		if k8sCluster.Master.Node != nodeID {
			newDeployments[nodeID] = gridtypes.Deployment{}
		}
		for _, worker := range k8sCluster.Workers {
			if worker.Node != nodeID {
				newDeployments[nodeID] = gridtypes.Deployment{}
			}
		}
	}

	k8sCluster.NodeDeploymentID, err = k.deployer.Cancel(ctx, oldDeployments, newDeployments)
	delete(k.tfPluginClient.StateLoader.currentNodeDeployment, k8sCluster.Master.Node)
	for _, worker := range k8sCluster.Workers {
		delete(k.tfPluginClient.StateLoader.currentNodeDeployment, worker.Node)
	}
	return err
}

// InvalidateBrokenAttributes removes outdated attrs and deleted contracts
func (k *K8sDeployer) InvalidateBrokenAttributes(k8sCluster *workloads.K8sCluster) error {
	sub := k.tfPluginClient.SubstrateConn
	newWorkers := make([]workloads.K8sNodeData, 0)
	validNodes := make(map[uint32]struct{})
	for node, contractID := range k8sCluster.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.State.IsCreated) || errors.Is(err, substrate.ErrNotFound) {
			delete(k8sCluster.NodeDeploymentID, node)
			delete(k8sCluster.NodesIPRange, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		} else {
			validNodes[node] = struct{}{}
		}

	}
	if _, ok := validNodes[k8sCluster.Master.Node]; !ok {
		k8sCluster.Master = &workloads.K8sNodeData{}
	}
	for _, worker := range k8sCluster.Workers {
		if _, ok := validNodes[worker.Node]; ok {
			newWorkers = append(newWorkers, worker)
		}
	}
	k8sCluster.Workers = newWorkers
	return nil
}
