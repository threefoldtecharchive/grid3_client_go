package deployer

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	substratemanager "github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type DeploymentManager interface {
	CancelAll() error
	Commit(ctx context.Context) error
	SetWorkloads(workloads map[uint32][]gridtypes.Workload) error
	CancelWorkloads(workloads map[uint32]map[string]bool) error
	GetWorkload(nodeID uint32, name string) (gridtypes.Workload, error)
	GetDeployment(nodeID uint32) (gridtypes.Deployment, error)
	GetContractIDs() map[uint32]uint64
}

type deploymentManager struct {

	identity            substrate.Identity
	twinID              uint32
	deploymentIDs       map[uint32]uint64 //TODO : should include all contracts of user
	affectedDeployments map[uint32]uint64
	plannedDeployments  map[uint32]gridtypes.Deployment
	gridClient          proxy.Client
	ncPool              client.NodeClientCollection
	substrate           subi.ManagerInterface
	//connection field
}

func NewDeploymentManager(
	identity substrate.Identity,
	twinID uint32,
	deploymentIDs map[uint32]uint64,
	gridClient proxy.Client,
	ncPool client.NodeClientCollection,
	sub subi.ManagerInterface) DeploymentManager {

	return &deploymentManager{
		identity,
		twinID,
		deploymentIDs,
		make(map[uint32]uint64),
		make(map[uint32]gridtypes.Deployment), ///2 lines down
		make(map[string]uint64),
		make([]string, 0),
		gridClient,
		ncPool,
		sub,
	}
}

func (d *deploymentManager) CancelAll() error { //TODO
	sub, err := d.substrate.SubstrateExt()
	if err != nil {
		return errors.Wrapf(err, "couldn't get substrate ")
	}
	for _, contractID := range d.deploymentIDs {
		err = sub.CancelContract(d.identity, contractID)
		if err != nil {
			return errors.Wrapf(err, "couldn't cancel contract with id %d", contractID)
		}
	}
	d.deploymentIDs = make(map[uint32]uint64)
	d.affectedDeployments = make(map[uint32]uint64)
	return nil
}

func (d *deploymentManager) CancelWorkloads(workloads map[uint32]map[string]bool) error {
	// deployments with cancelled workloads should be added to affected deployments and planned deployments
	// if a planned deployment had a cancelled workload, and now is empty, it should be removed from cancelled workloads

	// workloads are not cancelled until a user commits changes
	log.Printf("workloads to cancel: %+v", workloads)

	planned := map[uint32]gridtypes.Deployment{}
	for k, v := range d.plannedDeployments {
		planned[k] = v
	}
	affected := map[uint32]uint64{}
	for k, v := range d.affectedDeployments {
		affected[k] = v
	}
	contracts := d.GetContractIDs()
	for nodeID, cancelledWorkloads := range workloads {
		affected[nodeID] = contracts[nodeID]
		dl, err := d.GetDeployment(nodeID)
		if err != nil {
			return errors.Wrapf(err, "couldn't get deployment at node %d", nodeID)
		}
		for idx := 0; idx < len(dl.Workloads); {
			wlName := dl.Workloads[idx].Name.String()
			lastIdx := len(dl.Workloads) - 1
			if _, ok := cancelledWorkloads[wlName]; ok {
				dl.Workloads[idx], dl.Workloads[lastIdx] = dl.Workloads[lastIdx], dl.Workloads[idx]
				dl.Workloads = dl.Workloads[:lastIdx]
			} else {
				idx++
			}
		}
		if len(dl.Workloads) > 0 {
			planned[nodeID] = dl
		}
	}
	d.plannedDeployments = planned
	d.affectedDeployments = affected
	return nil
}

func getNodeSubnets(d gridtypes.Deployment) (map[string]string, error) {
	subnets := map[string]string{}
	for _, wl := range d.Workloads {
		if wl.Type != zos.NetworkType {
			continue
		}
		dataI, err := wl.WorkloadData()
		if err != nil {
			return map[string]string{}, errors.Wrap(err, "failed to get workload data")
		}
		data, ok := dataI.(*zos.Network)
		if !ok {
			return map[string]string{}, errors.New("couldn't cast workload data")
		}
		subnets[wl.Name.String()] = data.Subnet.String()
	}
	return subnets, nil
}

func getUsedIPs(d gridtypes.Deployment) (map[string]map[string]bool, error) {
	usedIPs := map[string]map[string]bool{}
	for _, wl := range d.Workloads {
		if wl.Type != zos.ZMachineType {
			continue
		}
		dataI, err := wl.WorkloadData()
		if err != nil {
			return map[string]map[string]bool{}, errors.Wrap(err, "failed to get workload data")
		}
		data, ok := dataI.(*zos.ZMachine)
		if !ok {
			return map[string]map[string]bool{}, errors.New("couldn't cast workload data")
		}
		ip := data.Network.Interfaces[0].IP.String()
		if ip == "" {
			continue
		}
		networkName := data.Network.Interfaces[0].Network.String()
		if _, ok := usedIPs[networkName]; !ok {
			usedIPs[networkName] = map[string]bool{}
		}
		usedIPs[networkName][ip] = true
	}
	return usedIPs, nil
}

func (d *deploymentManager) assignVMIPs() error {

	// if there is a k8s cluster, master node ip should be assigned in workers' env vars
	masterIPs := map[uint32]map[string]string{}

	for nodeID, deployment := range d.plannedDeployments {
		subnets, err := getNodeSubnets(deployment)
		if err != nil {
			return err
		}
		usedIPs, err := getUsedIPs(deployment)
		if err != nil {
			return err
		}
		for idx, wl := range deployment.Workloads {
			if wl.Type != zos.ZMachineType {
				continue
			}
			dataI, err := wl.WorkloadData()
			if err != nil {
				return errors.Wrap(err, "failed to get workload data")
			}
			data, ok := dataI.(*zos.ZMachine)
			if !ok {
				return errors.New("couldn't cast workload data")
			}
			ip := data.Network.Interfaces[0].IP

			networkName := data.Network.Interfaces[0].Network.String()
			_, cidr, err := net.ParseCIDR(subnets[networkName])
			if err != nil {
				return errors.Wrapf(err, "invalid ip %s", subnets[networkName])
			}

			if ip != nil && cidr.Contains(net.ParseIP(ip.String())) {
				// this vm already has a valid assigned ip
				continue
			}
			cur := byte(2)
			ip = cidr.IP
			ip[3] = cur
			for {
				if _, ok := usedIPs[networkName][ip.String()]; !ok {
					break
				}
				if cur == 254 {
					return errors.New("all 253 ips of the network are exhausted")
				}
				cur++
				ip[3] = cur
			}
			data.Network.Interfaces[0].IP = ip
			if s, ok := data.Env["K3S_URL"]; ok {
				if s == "" {
					if _, ok := masterIPs[nodeID]; !ok {
						masterIPs[nodeID] = make(map[string]string)
					}
					masterIPs[nodeID][wl.Name.String()] = ip.String()
				}
			}

			deployment.Workloads[idx].Data = gridtypes.MustMarshal(data)
			usedIPs[networkName][ip.String()] = true
		}

	}
	// assign k8s worker ips
	for _, deployment := range d.plannedDeployments {
		for idx, wl := range deployment.Workloads {
			if wl.Type != zos.ZMachineType {
				continue
			}
			dataI, err := wl.WorkloadData()
			if err != nil {
				return errors.Wrap(err, "failed to get workload data")
			}
			data, ok := dataI.(*zos.ZMachine)
			if !ok {
				return errors.New("couldn't cast workload data")
			}
			if s, ok := data.Env["K3S_URL"]; ok {
				if s != "" {
					master := strings.Split(s, ":")
					masterNodeID, err := strconv.Atoi(master[0])
					if err != nil {
						return err
					}
					masterName := master[1]
					data.Env["K3S_URL"] = fmt.Sprintf("https://%s:6443", masterIPs[uint32(masterNodeID)][masterName])
					deployment.Workloads[idx].Data = gridtypes.MustMarshal(data)
				}
			}
		}
	}
	return nil
}

func (d *deploymentManager) Commit(ctx context.Context) error {
	// generate gridtypes.Deployment from plannedDeployments
	deployer := NewDeployer(d.identity, d.twinID, d.gridClient, d.ncPool, true)
	s, err := d.substrate.SubstrateExt()
	if err != nil {
		return errors.Wrap(err, "couldn't get substrate client")
	}
	defer s.Close()
	createdNameContracts := map[string]uint64{}
	err = createNameContracts(createdNameContracts, *d, s)
	if err != nil {
		// revert changes
		revErr := cancelNameContracts(createdNameContracts, *d, s)
		if revErr != nil {
			return errors.Wrapf(revErr, "couldn't revert changes")
		}
		return err
	}

	err = d.assignVMIPs()
	if err != nil {
		return err
	}
	committedDeploymentsIDs, err := deployer.Deploy(ctx, s, d.affectedDeployments, d.plannedDeployments)
	if err != nil {
		return err
	}
	d.updateDeploymentIDs(committedDeploymentsIDs)
	d.affectedDeployments = make(map[uint32]uint64)
	d.plannedDeployments = make(map[uint32]gridtypes.Deployment)
	/////
	d.plannedNameContracts = make([]string, 0)
	////
	return nil
}

func (d *deploymentManager) SetWorkloads(workloads map[uint32][]gridtypes.Workload) error {
	planned := map[uint32]gridtypes.Deployment{}
	for k, v := range d.plannedDeployments {
		planned[k] = v
	}
	affected := map[uint32]uint64{}
	for k, v := range d.affectedDeployments {
		affected[k] = v
	}
	for nodeID, workloadsArray := range workloads {

		// move workload to planned deployments
		dl := gridtypes.Deployment{
			Version: 0,
			TwinID:  d.twinID,
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: d.twinID,
						Weight: 1,
					},
				},
			},
			Workloads: []gridtypes.Workload{},
		}

		if pdCopy, ok := planned[nodeID]; ok {
			dl = pdCopy
		} else if dID, ok := d.deploymentIDs[nodeID]; ok {
			s, err := d.substrate.SubstrateExt()
			if err != nil {
				return errors.Wrap(err, "couldn't get substrate client")
			}
			defer s.Close()
			nodeClient, err := d.ncPool.GetNodeClient(s, nodeID)
			if err != nil {
				return errors.Wrapf(err, "couldn't get node client: %d", nodeID)
			}
			// TODO: check if deployment exist on deploymentIDs and doesn't exist on node
			// TODO: use context from setWorkload
			dl, err = nodeClient.DeploymentGet(context.Background(), dID)
			if err != nil {
				return errors.Wrapf(err, "couldn't get deployment from node %d", nodeID)
			}
			affected[nodeID] = dl.ContractID
		}
		for idx := 0; idx < len(workloadsArray); {
			if workloadsArray[idx].Type == zos.GatewayNameProxyType {
				// if this is a gatewayNameProxy worklaod, stage name contract
				d.plannedNameContracts = append(d.plannedNameContracts, workloadsArray[idx].Name.String())
			}
			if workload, err := dl.Get(workloadsArray[idx].Name); err == nil {
			workloadWithID, err := dl.Get(workloadsArray[idx].Name)
			if err == nil {
				//override existing workload
				*(workloadWithID.Workload) = workloadsArray[idx]

				swap := reflect.Swapper(workloadsArray)
				swap(idx, len(workloadsArray)-1)
				workloadsArray = workloadsArray[:len(workloadsArray)-1]
			} else {
				idx++
			}

		}
		dl.Workloads = append(dl.Workloads, workloadsArray...)
		planned[nodeID] = dl
	}
	d.plannedDeployments = planned
	d.affectedDeployments = affected

	return nil
}

func (d *deploymentManager) GetWorkload(nodeID uint32, name string) (gridtypes.Workload, error) {
	if deployment, ok := d.deploymentIDs[nodeID]; ok {
		s, err := d.substrate.SubstrateExt()
		if err != nil {
			return gridtypes.Workload{}, errors.Wrap(err, "couldn't get substrate client")
		}
		defer s.Close()
		nodeClient, err := d.ncPool.GetNodeClient(s, nodeID)
		if err != nil {
			return gridtypes.Workload{}, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}
		dl, err := nodeClient.DeploymentGet(context.Background(), deployment)
		if err != nil {
			return gridtypes.Workload{}, errors.Wrapf(err, "couldn't get deployment from node %d", nodeID)
		}

		for _, workload := range dl.Workloads {
			if workload.Name == gridtypes.Name(name) {
				return workload, nil
			}
		}
		return gridtypes.Workload{}, fmt.Errorf("couldn't get workload with name %s", name)
	}
	return gridtypes.Workload{}, fmt.Errorf("couldn't get deployment with node ID %d", nodeID)

}

func (d *deploymentManager) GetDeployment(nodeID uint32) (gridtypes.Deployment, error) {
	dl := gridtypes.Deployment{}
	if dID, ok := d.deploymentIDs[nodeID]; ok {
		s, err := d.substrate.SubstrateExt()
		if err != nil {
			return gridtypes.Deployment{}, errors.Wrap(err, "couldn't get substrate client")
		}
		defer s.Close()
		nodeClient, err := d.ncPool.GetNodeClient(s, nodeID)
		if err != nil {
			return gridtypes.Deployment{}, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}
		dl, err = nodeClient.DeploymentGet(context.Background(), dID)
		if err != nil {
			return gridtypes.Deployment{}, errors.Wrapf(err, "couldn't get deployment from node %d", nodeID)
		}
		return dl, nil
	}
	return gridtypes.Deployment{}, fmt.Errorf("couldn't get deployment with node ID %d", nodeID)
}

func (d *deploymentManager) GetContractIDs() map[uint32]uint64 {
	return d.deploymentIDs
}

func (d *deploymentManager) updateDeploymentIDs(committedDeploymentsIDs map[uint32]uint64) {
	for k, v := range committedDeploymentsIDs {
		d.deploymentIDs[k] = v
	}
	for k := range d.affectedDeployments {
		if _, ok := committedDeploymentsIDs[k]; !ok {
			delete(d.deploymentIDs, k)
		}
	}
}

/////
func createNameContracts(createdNameContracts map[string]uint64, d deploymentManager, sub subi.SubstrateExt) error {
	for _, contractName := range d.plannedNameContracts {
		var contractID uint64
		if _, ok := d.nameContracts[contractName]; ok {
			id, err := sub.InvalidateNameContract(context.Background(), d.identity, d.nameContracts[contractName], contractName)
			if err != nil {
				return err
			}
			contractID = id
		}
		if contractID == 0 {
			id, err := sub.CreateNameContract(d.identity, contractName)
			if err != nil {
				return err
			}
			contractID = id
			createdNameContracts[contractName] = id
		}
	}
	return nil
}

func cancelNameContracts(createdNameContracts map[string]uint64, d deploymentManager, sub subi.SubstrateExt) error {
	for _, id := range createdNameContracts {
		err := sub.CancelContract(d.identity, id)
		if err != nil {
			return err
		}
	}
	return nil
}
