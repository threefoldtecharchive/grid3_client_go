package deployer

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	client "github.com/threefoldtech/grid3-go/node"
	substratemanager "github.com/threefoldtech/grid3-go/substrate_manager"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
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

	SetWorkload(nodeID uint32, workload gridtypes.Workload) error
	GetWorkload(nodeID uint32, name string) (gridtypes.Workload, error)
	GetDeployment(nodeID uint32) (gridtypes.Deployment, error)
}

type deploymentManager struct {
	identity            substrate.Identity
	twinID              uint32
	deploymentIDs       map[uint32]uint64 //TODO : should include all contracts of user
	affectedDeployments map[uint32]uint64
	plannedDeployments  map[uint32]gridtypes.Deployment
	gridClient          proxy.Client
	ncPool              client.NodeClientCollection
	substrate           substratemanager.ManagerInterface
	//connection field
}

func NewDeploymentManager(identity substrate.Identity, twinID uint32, deploymentIDs map[uint32]uint64, gridClient proxy.Client, ncPool client.NodeClientCollection, sub substratemanager.ManagerInterface) DeploymentManager {

	return &deploymentManager{
		identity,
		twinID,
		deploymentIDs,
		make(map[uint32]uint64),
		make(map[uint32]gridtypes.Deployment),
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
	d.deploymentIDs, err = deployer.Deploy(ctx, s, d.affectedDeployments, d.plannedDeployments)
	if err != nil {
		return err
	}
	log.Debug().Msgf("Deployed %+v", d.deploymentIDs)
	d.affectedDeployments = make(map[uint32]uint64)
	d.plannedDeployments = make(map[uint32]gridtypes.Deployment)
	return nil
}

func (d *deploymentManager) SetWorkload(nodeID uint32, workload gridtypes.Workload) error {
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

	if _, err := dl.Get(workload.Name); err == nil {
		return fmt.Errorf("workload name already exists: %s", workload.Name)
	}
	dl.Workloads = append(dl.Workloads, workload)
	d.plannedDeployments[nodeID] = dl
	return nil
}

func (d *deploymentManager) GetWorkload(nodeID uint32, name string) (gridtypes.Workload, error) {
	w := gridtypes.Workload{}
	if deployment, ok := d.deploymentIDs[nodeID]; ok {
		s, err := d.substrate.SubstrateExt()
		if err != nil {
			return w, errors.Wrap(err, "couldn't get substrate client")
		}
		defer s.Close()
		nodeClient, err := d.ncPool.GetNodeClient(s, nodeID)
		if err != nil {
			return w, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}
		dl, err := nodeClient.DeploymentGet(context.Background(), deployment)
		if err != nil {
			return w, errors.Wrapf(err, "couldn't get deployment from node %d", nodeID)
		}

		for _, workload := range dl.Workloads {
			if workload.Name == gridtypes.Name(name) {
				return workload, nil
			}
		}
		return w, fmt.Errorf("couldn't get workload with name %s", name)
	}
	return w, fmt.Errorf("couldn't get deployment with node ID %d", nodeID)

}

func (d *deploymentManager) GetDeployment(nodeID uint32) (gridtypes.Deployment, error) {
	return gridtypes.Deployment{}, nil
}
