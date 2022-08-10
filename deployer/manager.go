package deployer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	substratemanager "github.com/threefoldtech/grid3-go/substrate_manager"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
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
	identity           substrate.Identity
	twinID             uint32
	deploymentIDs      map[uint32]uint64
	deployments        map[uint32]gridtypes.Deployment
	plannedDeployments map[uint32]gridtypes.Deployment
	nodeClient         *client.NodeClient
	substrate          substratemanager.Manager
	//connection field
}

func NewDeploymentManager(identity substrate.Identity, twinID uint32, sub substratemanager.Manager, nodeClient *client.NodeClient) DeploymentManager {

	return &deploymentManager{
		identity,
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
	for nodeID, deployment := range d.plannedDeployments {
		h, err := deployment.ChallengeHash()
		if err != nil {
			return err
		}
		hex := hex.EncodeToString(h)
		err = deployment.Sign(d.twinID, d.identity)
		if err != nil {
			return err
		}
		sub, err := d.substrate.SubstrateExt()
		if err != nil {
			return err
		}
		pubIPCount := countDeploymentPublicIPs(deployment)
		contractID, err := sub.CreateNodeContract(d.identity, nodeID, nil, hex, pubIPCount)
		deployment.ContractID = contractID
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		err = d.nodeClient.DeploymentDeploy(ctx, deployment)
		if err != nil {
			rerr := sub.EnsureContractCanceled(d.identity, contractID)
			if rerr != nil {
				return fmt.Errorf("error sending deployment to the node: %w, error cancelling contract: %s; you must cancel it manually (id: %d)", err, rerr, contractID)
			} else {
				return errors.New("error sending deployment to the node")
			}
		}
		d.deployments[nodeID] = deployment
		d.deploymentIDs[nodeID] = deployment.ContractID
	}
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

func countDeploymentPublicIPs(dl gridtypes.Deployment) uint32 {
	var res uint32 = 0
	for _, wl := range dl.Workloads {
		if wl.Type == zos.PublicIPType {
			data, err := wl.WorkloadData()
			if err != nil {
				log.Printf("couldn't parse workload data %s", err.Error())
				continue
			}
			if data.(*zos.PublicIP).V4 {
				res++
			}
		}
	}
	return res
}
