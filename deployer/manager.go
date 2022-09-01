package deployer

import (
	"context"
	"fmt"
	"reflect"

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
	// CancelAll clears deployments, deploymentIDs, and deployments
	CancelAll() error
	// CancelNodeDeployment removes the entry from deployments, deploymentIDs, and deployments
	// CancelNodeDeployment(nodeID uint32)
	// Commit loads initDeployments from deploymentIDs which wasn't loaded previously
	Commit(ctx context.Context) error
	// SetWorkload adds the workload to the deployment associated with the node
	//             it should load the deployment in initDeployments if it exists in deploymentIDs and not loaded
	//             and return an error if the node is down for example

	SetWorkloads(workoads map[uint32][]gridtypes.Workload) error
	GetWorkload(nodeID uint32, name string) (gridtypes.Workload, error)
	GetDeployment(nodeID uint32) (gridtypes.Deployment, error)
	GetContractIDs() map[uint32]uint64
}

type deploymentManager struct {
	// identity            substrate.Identity
	// twinID              uint32
	// deploymentIDs       map[uint32]uint64 //TODO : should include all contracts of user
	// affectedDeployments map[uint32]uint64
	// plannedDeployments  map[uint32]gridtypes.Deployment
	// gridClient          proxy.Client
	// ncPool              client.NodeClientCollection
	// substrate           substratemanager.ManagerInterface
	// //connection field
	identity             substrate.Identity
	twinID               uint32
	deploymentIDs        map[uint32]uint64
	affectedDeployments  map[uint32]uint64
	plannedDeployments   map[uint32]gridtypes.Deployment
	nameContracts        map[string]uint64
	plannedNameContracts []string
	gridClient           proxy.Client
	ncPool               client.NodeClientCollection
	substrate            subi.ManagerInterface
}

func NewDeploymentManager(identity substrate.Identity, twinID uint32, deploymentIDs map[uint32]uint64, gridClient proxy.Client, ncPool client.NodeClientCollection, sub substratemanager.ManagerInterface) DeploymentManager {

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

func (d *deploymentManager) Commit(ctx context.Context) error {
	// generate gridtypes.Deployment from plannedDeployments
	deployer := NewDeployer(d.identity, d.twinID, d.gridClient, d.ncPool, true)
	s, err := d.substrate.SubstrateExt()
	if err != nil {
		return errors.Wrap(err, "couldn't get substrate client")
	}
	defer s.Close()
	/////////////////////
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
	////////////////////////////
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

	for nodeID, workloadsArray := range workloads {

		// move workload to planned deployments
		dl := gridtypes.Deployment{
			Version: 0,
			TwinID:  d.twinID,
			SignatureRequirement: gridtypes.SignatureRequirement{
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: d.twinID,
						Weight: 1,
					},
				},
				WeightRequired: 1,
			},
			Workloads: []gridtypes.Workload{},
		}

		if pdCopy, ok := d.plannedDeployments[nodeID]; ok {
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
			d.affectedDeployments[nodeID] = dl.ContractID
		}

		for idx := 0; idx < len(workloadsArray); {
			/////
			if workloadsArray[idx].Type == zos.GatewayNameProxyType {
				// if this is a gatewayNameProxy worklaod, stage name contract
				d.plannedNameContracts = append(d.plannedNameContracts, workloadsArray[idx].Name.String())
			}
			////
			if workload, err := dl.Get(workloadsArray[idx].Name); err == nil {
				//override existing workload
				workload.Data = workloadsArray[idx].Data
				workload.Description = workloadsArray[idx].Description
				workload.Metadata = workloadsArray[idx].Metadata
				workload.Result = workloadsArray[idx].Result
				workload.Type = workloadsArray[idx].Type
				workload.Version += 1

				swap := reflect.Swapper(workloadsArray)
				swap(idx, len(workloadsArray)-1)
				workloadsArray = workloadsArray[:len(workloadsArray)-1]

			} else {
				idx++
			}

		}
		dl.Workloads = append(dl.Workloads, workloadsArray...)
		d.plannedDeployments[nodeID] = dl
	}

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
