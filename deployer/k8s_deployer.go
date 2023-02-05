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
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	wl "github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type K8sDeployer struct {
	Master           *wl.K8sNodeData
	Workers          []wl.K8sNodeData
	NodesIPRange     map[uint32]gridtypes.IPNet
	Token            string
	SSHKey           string
	NetworkName      string
	NodeDeploymentID map[uint32]uint64

	TFPluginClient *TFPluginClient
	NodeUsedIPs    map[uint32][]byte
	ncPool         *client.NodeClientPool
	deployer       Deployer
}

func NewK8sDeployer(tfPluginClient *TFPluginClient) K8sDeployer {
	k8sDeployer := K8sDeployer{
		ncPool:         client.NewNodeClientPool(tfPluginClient.RMB),
		TFPluginClient: tfPluginClient,
		deployer:       Deployer{},
	}

	k8sDeployer.TFPluginClient = tfPluginClient
	k8sDeployer.deployer = NewDeployer(*tfPluginClient, true)

	return k8sDeployer
}

func (k *K8sDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt) error {
	newWorkers := make([]wl.K8sNodeData, 0)
	validNodes := make(map[uint32]struct{})
	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, substrate.ErrNotFound) {
			delete(k.NodeDeploymentID, node)
			delete(k.NodesIPRange, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		} else {
			validNodes[node] = struct{}{}
		}

	}
	if _, ok := validNodes[k.Master.Node]; !ok {
		k.Master = &wl.K8sNodeData{}
	}
	for _, worker := range k.Workers {
		if _, ok := validNodes[worker.Node]; ok {
			newWorkers = append(newWorkers, worker)
		}
	}
	k.Workers = newWorkers
	return nil
}

func (d *K8sDeployer) retainChecksums(workers []interface{}, master interface{}) {
	checksumMap := make(map[string]string)
	checksumMap[d.Master.Name] = d.Master.FlistChecksum
	for _, w := range d.Workers {
		checksumMap[w.Name] = w.FlistChecksum
	}
	typed := master.(map[string]interface{})
	typed["flist_checksum"] = checksumMap[typed["name"].(string)]
	for _, w := range workers {
		typed := w.(map[string]interface{})
		typed["flist_checksum"] = checksumMap[typed["name"].(string)]
	}
}

func (k *K8sDeployer) assignNodesIPs() error {
	masterNodeRange := k.NodesIPRange[k.Master.Node]
	if k.Master.IP == "" || !masterNodeRange.Contains(net.ParseIP(k.Master.IP)) {
		ip, err := k.getK8sFreeIP(masterNodeRange, k.Master.Node)
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for master")
		}
		k.Master.IP = ip
	}
	for idx, w := range k.Workers {
		workerNodeRange := k.NodesIPRange[w.Node]
		if w.IP != "" && workerNodeRange.Contains(net.ParseIP(w.IP)) {
			continue
		}
		ip, err := k.getK8sFreeIP(workerNodeRange, w.Node)
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for worker")
		}
		k.Workers[idx].IP = ip
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

func (k *K8sDeployer) GenerateVersionlessDeployments(ctx context.Context) (map[uint32]gridtypes.Deployment, error) {
	err := k.assignNodesIPs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to assign node ips")
	}
	deployments := make(map[uint32]gridtypes.Deployment)
	nodeWorkloads := make(map[uint32][]gridtypes.Workload)
	k8sCluster := wl.K8sCluster{
		Master:      k.Master,
		Workers:     k.Workers,
		Token:       k.Token,
		SSHKey:      k.SSHKey,
		NetworkName: k.NetworkName,
	}
	masterWorkloads := k.Master.ZosWorkload(&k8sCluster, true)
	nodeWorkloads[k.Master.Node] = append(nodeWorkloads[k.Master.Node], masterWorkloads...)
	for _, w := range k.Workers {
		workerWorkloads := w.ZosWorkload(&k8sCluster, true)
		nodeWorkloads[w.Node] = append(nodeWorkloads[w.Node], workerWorkloads...)
	}

	for node, ws := range nodeWorkloads {
		dl := gridtypes.Deployment{
			Version: 0,
			TwinID:  uint32(k.TFPluginClient.TwinID), //LocalTwin,
			// this contract id must match the one on substrate
			Workloads: ws,
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: k.TFPluginClient.TwinID,
						Weight: 1,
					},
				},
			},
		}
		deployments[node] = dl
	}
	return deployments, nil
}

func (d *K8sDeployer) validateChecksums() error {
	nodes := append(d.Workers, *d.Master)
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

func (k *K8sDeployer) ValidateNames(ctx context.Context) error {

	names := make(map[string]bool)
	names[k.Master.Name] = true
	for _, w := range k.Workers {
		if _, ok := names[w.Name]; ok {
			return fmt.Errorf("k8s workers and master must have unique names: %s occurred more than once", w.Name)
		}
		names[w.Name] = true
	}
	return nil
}

func (k *K8sDeployer) ValidateIPranges(ctx context.Context) error {

	if _, ok := k.NodesIPRange[k.Master.Node]; !ok {
		return fmt.Errorf("the master node %d doesn't exist in the network's ip ranges", k.Master.Node)
	}
	for _, w := range k.Workers {
		if _, ok := k.NodesIPRange[w.Node]; !ok {
			return fmt.Errorf("the node with id %d in worker %s doesn't exist in the network's ip ranges", w.Node, w.Name)
		}
	}
	return nil
}

func (k *K8sDeployer) validateToken(ctx context.Context) error {
	if k.Token == "" {
		return errors.New("empty token is now allowed")
	}

	is_alphanumeric := regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(k.Token)
	if !is_alphanumeric {
		return errors.New("token should be alphanumeric")
	}

	return nil
}

func (k *K8sDeployer) Validate(ctx context.Context, sub subi.SubstrateExt) error {
	if err := k.validateToken(ctx); err != nil {
		return err
	}
	if err := k.ValidateNames(ctx); err != nil {
		return err
	}
	if err := k.ValidateIPranges(ctx); err != nil {
		return err
	}
	nodes := make([]uint32, 0)
	nodes = append(nodes, k.Master.Node)
	for _, w := range k.Workers {
		nodes = append(nodes, w.Node)

	}
	return client.AreNodesUp(ctx, sub, nodes, k.ncPool)
}

func (k *K8sDeployer) Deploy(ctx context.Context, sub subi.SubstrateExt) error {
	if err := k.validateChecksums(); err != nil {
		return err
	}
	newDeployments, err := k.GenerateVersionlessDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}
	deploymentData := workloads.DeploymentData{
		Name: k.Master.Name,
		Type: "k8s",
	}
	newDeploymentsData := make(map[uint32]workloads.DeploymentData) ///////todo
	newDeploymentsData[k.Master.Node] = deploymentData
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	newDeploymentsSolutionProvider[k.Master.Node] = nil
	currentDeployments, err := k.deployer.Deploy(ctx, k.NodeDeploymentID, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)
	if err := k.updateState(ctx, sub, currentDeployments); err != nil {
		log.Printf("error updating state: %s\n", err)
	}
	return err
}

func (k *K8sDeployer) updateState(ctx context.Context, sub subi.SubstrateExt, currentDeploymentIDs map[uint32]uint64) error {
	log.Printf("current deployments\n")
	k.NodeDeploymentID = currentDeploymentIDs
	currentDeployments, err := k.deployer.GetDeployments(ctx, currentDeploymentIDs)
	if err != nil {
		return errors.Wrap(err, "failed to get deployments to update local state")
	}

	err = printDeployments(currentDeployments)
	if err != nil {
		return errors.Wrap(err, "couldn't print deployments data")
	}

	publicIPs := make(map[string]string)
	publicIP6s := make(map[string]string)
	yggIPs := make(map[string]string)
	privateIPs := make(map[string]string)
	for _, dl := range currentDeployments {
		for _, w := range dl.Workloads {
			if w.Type == zos.PublicIPType {
				d := zos.PublicIPResult{}
				if err := json.Unmarshal(w.Result.Data, &d); err != nil {
					log.Printf("error unmarshalling json: %s\n", err)
					continue
				}
				publicIPs[string(w.Name)] = d.IP.String()
				publicIP6s[string(w.Name)] = d.IPv6.String()
			} else if w.Type == zos.ZMachineType {
				d, err := w.WorkloadData()
				if err != nil {
					log.Printf("error loading machine data: %s\n", err)
					continue
				}
				privateIPs[string(w.Name)] = d.(*zos.ZMachine).Network.Interfaces[0].IP.String()

				var result zos.ZMachineResult
				if err := w.Result.Unmarshal(&result); err != nil {
					log.Printf("error loading machine result: %s\n", err)
				}
				yggIPs[string(w.Name)] = result.YggIP
			}
		}
	}
	masterIPName := fmt.Sprintf("%sip", k.Master.Name)
	k.Master.ComputedIP = publicIPs[masterIPName]
	k.Master.ComputedIP6 = publicIP6s[masterIPName]
	k.Master.IP = privateIPs[string(k.Master.Name)]
	k.Master.YggIP = yggIPs[string(k.Master.Name)]

	for idx, w := range k.Workers {
		workerIPName := fmt.Sprintf("%sip", w.Name)
		k.Workers[idx].ComputedIP = publicIPs[workerIPName]
		k.Workers[idx].ComputedIP = publicIP6s[workerIPName]
		k.Workers[idx].IP = privateIPs[string(w.Name)]
		k.Workers[idx].YggIP = yggIPs[string(w.Name)]
	}
	// k.updateNetworkState(d, k.TFP)
	log.Printf("Current state after updatestate %v\n", k)
	return nil
}

// func (k *K8sDeployer) updateNetworkState(state state.StateI) {
// 	ns := state.GetNetworkState()
// 	network := ns.GetNetwork(k.NetworkName)
// 	before, _ := d.GetChange("node_deployment_id")
// 	for node, deploymentID := range before.(map[string]interface{}) {
// 		nodeID, err := strconv.Atoi(node)
// 		if err != nil {
// 			log.Printf("error converting node id string to int: %+v", err)
// 			continue
// 		}
// 		deploymentIDStr := fmt.Sprint(deploymentID.(int))
// 		network.DeleteDeployment(uint32(nodeID), deploymentIDStr)
// 	}
// 	// remove old ips
// 	network.DeleteDeployment(k.Master.Node, fmt.Sprint(k.NodeDeploymentID[k.Master.Node]))
// 	for _, worker := range k.Workers {
// 		network.DeleteDeployment(worker.Node, fmt.Sprint(k.NodeDeploymentID[worker.Node]))
// 	}

// 	// append new ips
// 	masterNodeIPs := network.GetDeploymentIPs(k.Master.Node, fmt.Sprint(k.NodeDeploymentID[k.Master.Node]))
// 	masterIP := net.ParseIP(k.Master.IP)
// 	if masterIP == nil {
// 		log.Printf("couldn't parse master ip")
// 	} else {
// 		masterNodeIPs = append(masterNodeIPs, masterIP.To4()[3])
// 	}
// 	network.SetDeploymentIPs(k.Master.Node, fmt.Sprint(k.NodeDeploymentID[k.Master.Node]), masterNodeIPs)
// 	for _, worker := range k.Workers {
// 		workerNodeIPs := network.GetDeploymentIPs(worker.Node, fmt.Sprint(k.NodeDeploymentID[worker.Node]))
// 		workerIP := net.ParseIP(worker.IP)
// 		if workerIP == nil {
// 			log.Printf("couldn't parse worker ip at node (%d)", worker.Node)
// 		} else {
// 			workerNodeIPs = append(workerNodeIPs, workerIP.To4()[3])
// 		}
// 		network.SetDeploymentIPs(worker.Node, fmt.Sprint(k.NodeDeploymentID[worker.Node]), workerNodeIPs)
// 	}
// }

func printDeployments(dls map[uint32]gridtypes.Deployment) (err error) {
	for node, dl := range dls {
		log.Printf("node id: %d\n", node)
		enc := json.NewEncoder(log.Writer())
		enc.SetIndent("", "  ")
		err := enc.Encode(dl)
		if err != nil {
			return err
		}
	}

	return
}

func (k *K8sDeployer) removeDeletedContracts(ctx context.Context, sub subi.SubstrateExt) error {
	nodeDeploymentID := make(map[uint32]uint64)
	for nodeID, deploymentID := range k.NodeDeploymentID {
		cont, err := sub.GetContract(deploymentID)
		if err != nil {
			return errors.Wrap(err, "failed to get deployments")
		}
		if !cont.IsDeleted() {
			nodeDeploymentID[nodeID] = deploymentID
		}
	}
	k.NodeDeploymentID = nodeDeploymentID
	return nil
}

func (k *K8sDeployer) updateFromRemote(ctx context.Context, sub subi.SubstrateExt) error {
	if err := k.removeDeletedContracts(ctx, sub); err != nil {
		return errors.Wrap(err, "failed to remove deleted contracts")
	}
	currentDeployments, err := k.deployer.GetDeployments(ctx, k.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "failed to fetch remote deployments")
	}
	log.Printf("calling updateFromRemote")
	err = printDeployments(currentDeployments)
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
				if !keyUpdated && SSHKey != k.SSHKey {
					k.SSHKey = SSHKey
					keyUpdated = true
				}
				if !tokenUpdated && token != k.Token {
					k.Token = token
					tokenUpdated = true
				}
				if !networkUpdated && networkName != k.NetworkName {
					k.NetworkName = networkName
					networkUpdated = true
				}
			}
		}
	}

	nodeDeploymentID := make(map[uint32]uint64)
	for node, dl := range currentDeployments {
		nodeDeploymentID[node] = dl.ContractID
	}
	k.NodeDeploymentID = nodeDeploymentID
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
	masterNodeID, ok := workloadNodeID[k.Master.Name]
	if !ok {
		k.Master = nil
	} else {
		masterWorkload := workloadObj[k.Master.Name]
		masterIP := workloadComputedIP[k.Master.Name]
		masterIP6 := workloadComputedIP6[k.Master.Name]
		masterDiskSize := workloadDiskSize[k.Master.Name]

		m, err := wl.NewK8sNodeDataFromWorkload(masterWorkload, masterNodeID, masterDiskSize, masterIP, masterIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get master data from workload")
		}
		k.Master = &m
	}
	// update workers
	workers := make([]wl.K8sNodeData, 0)
	for _, w := range k.Workers {
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
		w, err := wl.NewK8sNodeDataFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get worker data from workload")
		}
		workers = append(workers, w)
	}
	// add missing workers (in case of failed deletions)
	for name, workerNodeID := range workloadNodeID {
		if name == k.Master.Name {
			continue
		}
		workerWorkload := workloadObj[name]
		workerIP := workloadComputedIP[name]
		workerIP6 := workloadComputedIP6[name]
		workerDiskSize := workloadDiskSize[name]
		w, err := wl.NewK8sNodeDataFromWorkload(workerWorkload, workerNodeID, workerDiskSize, workerIP, workerIP6)
		if err != nil {
			return errors.Wrap(err, "failed to get worker data from workload")
		}
		workers = append(workers, w)
	}
	k.Workers = workers
	log.Printf("after updateFromRemote\n")
	enc := json.NewEncoder(log.Writer())
	enc.SetIndent("", "  ")
	err = enc.Encode(k)
	if err != nil {
		return errors.Wrap(err, "failed to encode k8s deployer")
	}

	return nil
}
