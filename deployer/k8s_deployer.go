package deployer

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// K8s Deployer for deploying k8s
type K8sDeployer struct {
	NodeUsedIPs    map[uint32][]byte
	tfPluginClient *TFPluginClient
	deployer       Deployer
}

// Generate new K8s Deployer
func NewK8sDeployer(tfPluginClient *TFPluginClient) K8sDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	k8sDeployer := K8sDeployer{
		NodeUsedIPs:    map[uint32][]byte{},
		tfPluginClient: tfPluginClient,
		deployer:       deployer,
	}

	return k8sDeployer
}

func (k *K8sDeployer) validateToken(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if k8sCluster.Token == "" {
		return errors.New("empty token is now allowed")
	}

	is_alphanumeric := regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(k8sCluster.Token)
	if !is_alphanumeric {
		return errors.New("token should be alphanumeric")
	}

	return nil
}

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

	if err := validateAccountBalanceForExtrinsics(k.deployer.SubstrateConn, k.tfPluginClient.Identity); err != nil {
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
	for _, w := range k8sCluster.Workers {
		nodes = append(nodes, w.Node)
	}
	return client.AreNodesUp(ctx, sub, []uint32{k8sCluster.Master.Node}, k.tfPluginClient.NcPool)
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
	masterWorkloads := k8sCluster.Master.GenerateK8sWorkload(k8sCluster, true)
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

func (k *K8sDeployer) nodeIps(k8sCluster *workloads.K8sCluster) error {
	network := k.tfPluginClient.StateLoader.networks.getNetwork(k8sCluster.NetworkName)
	nodesIPRange := make(map[uint32]gridtypes.IPNet)
	var err error
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
	if err := k.nodeIps(k8sCluster); err != nil {
		return err
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
	newDeploymentsSolutionProvider[k8sCluster.Master.Node] = nil

	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment

	k8sCluster.NodeDeploymentID, err = k.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

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
	}
	k8sCluster.NodeDeploymentID, err = k.deployer.Cancel(ctx, oldDeployments, newDeployments)
	return err
}
