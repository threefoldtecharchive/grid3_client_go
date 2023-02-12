// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// K8sDeployer for deploying k8s
type K8sDeployer struct {
	NodeUsedIPs    map[uint32][]byte
	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// NewK8sDeployer generates new K8s Deployer
func NewK8sDeployer(tfPluginClient *TFPluginClient) K8sDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	k8sDeployer := K8sDeployer{
		NodeUsedIPs:    map[uint32][]byte{},
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}

	return k8sDeployer
}

// Validate validates K8s deployer
func (d *K8sDeployer) Validate(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	sub := d.tfPluginClient.SubstrateConn
	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.identity); err != nil {
		return err
	}

	if err := k8sCluster.ValidateToken(); err != nil {
		return err
	}

	if err := k8sCluster.ValidateNames(); err != nil {
		return err
	}

	if err := k8sCluster.ValidateIPranges(); err != nil {
		return err
	}

	if err := k8sCluster.ValidateChecksums(); err != nil {
		return err
	}

	// validate cluster nodes
	var nodes []uint32
	nodes = append(nodes, k8sCluster.Master.Node)
	for _, worker := range k8sCluster.Workers {
		if !workloads.Contains(nodes, worker.Node) {
			nodes = append(nodes, worker.Node)
		}
	}
	return client.AreNodesUp(ctx, sub, nodes, d.tfPluginClient.NcPool)
}

// GenerateVersionlessDeployments generates a new deployment without a version
func (d *K8sDeployer) GenerateVersionlessDeployments(ctx context.Context, k8sCluster *workloads.K8sCluster) (map[uint32]gridtypes.Deployment, error) {
	err := d.assignNodesIPs(k8sCluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	deployments := make(map[uint32]gridtypes.Deployment)
	nodeWorkloads := make(map[uint32][]gridtypes.Workload)

	masterWorkloads := k8sCluster.Master.MasterZosWorkload(k8sCluster)
	nodeWorkloads[k8sCluster.Master.Node] = append(nodeWorkloads[k8sCluster.Master.Node], masterWorkloads...)
	for _, w := range k8sCluster.Workers {
		workerWorkloads := w.WorkerZosWorkload(k8sCluster)
		nodeWorkloads[w.Node] = append(nodeWorkloads[w.Node], workerWorkloads...)
	}

	for node, ws := range nodeWorkloads {
		dl := workloads.NewGridDeployment(d.tfPluginClient.twinID, ws)
		dl.Metadata, err = k8sCluster.GenerateMetadata()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate deployment %s metadata", k8sCluster.Master.Name)
		}

		deployments[node] = dl
	}
	return deployments, nil
}

// Deploy deploys a k8s cluster deployment
func (d *K8sDeployer) Deploy(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if err := d.assignNodeIPRange(k8sCluster); err != nil {
		return err
	}

	err := k8sCluster.InvalidateBrokenAttributes(d.tfPluginClient.SubstrateConn)
	if err != nil {
		return err
	}

	if err := d.Validate(ctx, k8sCluster); err != nil {
		return err
	}

	newDeployments, err := d.GenerateVersionlessDeployments(ctx, k8sCluster)
	if err != nil {
		return errors.Wrap(err, "couldn't generate k8s grid deployments")
	}

	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	newDeploymentsSolutionProvider[k8sCluster.Master.Node] = nil

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment

	// TODO: use NodeDeploymentID instead of oldDeployments
	currentDeployments, err := d.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsSolutionProvider)

	// update deployments state
	if currentDeployments[k8sCluster.Master.Node] != 0 {
		if k8sCluster.NodeDeploymentID == nil {
			k8sCluster.NodeDeploymentID = make(map[uint32]uint64)
		}

		k8sCluster.NodeDeploymentID[k8sCluster.Master.Node] = currentDeployments[k8sCluster.Master.Node]
		d.tfPluginClient.StateLoader.currentNodeDeployment[k8sCluster.Master.Node] = currentDeployments[k8sCluster.Master.Node]
		for _, w := range k8sCluster.Workers {
			k8sCluster.NodeDeploymentID[w.Node] = currentDeployments[w.Node]
			d.tfPluginClient.StateLoader.currentNodeDeployment[w.Node] = currentDeployments[w.Node]
		}
	}

	return err
}

// Cancel cancels a k8s cluster deployment
func (d *K8sDeployer) Cancel(ctx context.Context, k8sCluster *workloads.K8sCluster) (err error) {
	if err := d.Validate(ctx, k8sCluster); err != nil {
		return err
	}
	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment
	for nodeID, contractID := range oldDeployments {
		if k8sCluster.Master.Node == nodeID {
			err = d.deployer.Cancel(ctx, contractID)
			if err != nil {
				return errors.Wrapf(err, "couldn't cancel master %s, contract %d", k8sCluster.Master.Name, contractID)
			}
			delete(d.tfPluginClient.StateLoader.currentNodeDeployment, nodeID)
			delete(k8sCluster.NodeDeploymentID, nodeID)
			continue
		}
		for _, worker := range k8sCluster.Workers {
			if worker.Node == nodeID {
				err = d.deployer.Cancel(ctx, contractID)
				if err != nil {
					return errors.Wrapf(err, "couldn't cancel worker %s, contract %d", worker.Name, contractID)
				}
				delete(d.tfPluginClient.StateLoader.currentNodeDeployment, nodeID)
				delete(k8sCluster.NodeDeploymentID, nodeID)
				break
			}
		}
	}

	return nil
}

// UpdateFromRemote update a k8s cluster
func (d *K8sDeployer) UpdateFromRemote(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	if err := d.removeDeletedContracts(ctx, k8sCluster); err != nil {
		return errors.Wrap(err, "failed to remove deleted contracts")
	}
	currentDeployments, err := d.deployer.GetDeployments(ctx, k8sCluster.NodeDeploymentID)
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

		m, err := workloads.NewK8sNodeFromWorkload(masterWorkload, masterNodeID, masterDiskSize, masterIP, masterIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get master node from workload")
		}
		k8sCluster.Master = &m
	}
	// update workers
	workers := make([]workloads.K8sNode, 0)
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
		w, err := workloads.NewK8sNodeFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
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
		w, err := workloads.NewK8sNodeFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get worker data from workload")
		}
		workers = append(workers, w)
	}
	k8sCluster.Workers = workers
	log.Printf("after updateFromRemote\n")
	enc := json.NewEncoder(log.Writer())
	enc.SetIndent("", "  ")
	err = enc.Encode(d)
	if err != nil {
		return errors.Wrap(err, "failed to encode k8s deployer")
	}

	return nil
}

func (d *K8sDeployer) removeDeletedContracts(ctx context.Context, k8sCluster *workloads.K8sCluster) error {
	sub := d.tfPluginClient.SubstrateConn
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

func (d *K8sDeployer) getK8sFreeIP(ipRange gridtypes.IPNet, nodeID uint32) (string, error) {
	ip := ipRange.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("the provided ip range (%s) is not a valid ipv4", ipRange.String())
	}

	for i := 2; i < 255; i++ {
		hostID := byte(i)
		if !workloads.Contains(d.NodeUsedIPs[nodeID], hostID) {
			d.NodeUsedIPs[nodeID] = append(d.NodeUsedIPs[nodeID], hostID)
			ip[3] = hostID
			return ip.String(), nil
		}
	}
	return "", errors.New("all ips are used")
}

func (d *K8sDeployer) assignNodesIPs(k8sCluster *workloads.K8sCluster) error {
	masterNodeRange := k8sCluster.NodesIPRange[k8sCluster.Master.Node]
	if k8sCluster.Master.IP == "" || !masterNodeRange.Contains(net.ParseIP(k8sCluster.Master.IP)) {
		ip, err := d.getK8sFreeIP(masterNodeRange, k8sCluster.Master.Node)
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
		ip, err := d.getK8sFreeIP(workerNodeRange, w.Node)
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for worker")
		}
		k8sCluster.Workers[idx].IP = ip
	}
	return nil
}

func (d *K8sDeployer) assignNodeIPRange(k8sCluster *workloads.K8sCluster) (err error) {
	network := d.tfPluginClient.StateLoader.networks.getNetwork(k8sCluster.NetworkName)
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
