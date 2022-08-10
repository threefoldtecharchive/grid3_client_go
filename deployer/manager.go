package deployer

import (
	"errors"

	substratemanager "github.com/threefoldtech/grid3-go/substrate_manager"
	"github.com/threefoldtech/zos/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

type DeploymentManager interface {
	// CancelAll clears deployments, deploymentIDs, and deployments
	CancelAll()
	// CancelNodeDeployment removes the entry from deployments, deploymentIDs, and deployments
	// CancelNodeDeployment(nodeID uint32)
	// Commit loads initDeployments from deploymentIDs which wasn't loaded previously
	Commit() error
	// SetWorkload adds the workload to the deployment associated with the node
	//             it should load the deployment in initDeployments if it exists in deploymentIDs and not loaded
	//             and return an error if the node is down for example
	SetWorkload(nodeID uint32, workload gridtypes.Workload) error
}

type deploymentManager struct {
	twinID             uint32
	deploymentIDs      map[uint32]uint64
	deployments        map[uint32]gridtypes.Deployment
	plannedDeployments map[uint32]gridtypes.Deployment
	nodeClient         *client.NodeClient
	substrate          substratemanager.Manager
	//connection field
}

func NewDeploymentManager(twinID uint32, sub substratemanager.Manager, nodeClient *client.NodeClient) DeploymentManager {

	return &deploymentManager{
		twinID,
		make(map[uint32]uint64),
		make(map[uint32]gridtypes.Deployment),
		make(map[uint32]gridtypes.Deployment),
		nodeClient,
		sub,
	}
}
func (d *deploymentManager) CancelAll() {

}

// func (d *deploymentManager) CancelNodeDeployment(nodeID uint32) {

// }
func (d *deploymentManager) Commit() error {
	// generate gridtypes.Deployment from plannedDeployments
	return nil

}

func (d *deploymentManager) SetWorkload(nodeID uint32, workload gridtypes.Workload) error {
	// move workload to planned deployments
	if pdCopy, ok := d.plannedDeployments[nodeID]; ok {
		for _, wl := range pdCopy.Workloads {
			if wl.Name == workload.Name {
				return errors.New("Workload names should be unique")
			}
		}
		pdCopy.Workloads = append(pdCopy.Workloads, workload)
		d.plannedDeployments[nodeID] = pdCopy

	} else {
		newDeployment := gridtypes.Deployment{
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
			Workloads: []gridtypes.Workload{workload},
		}
		d.plannedDeployments[nodeID] = newDeployment
	}
	return nil
}
