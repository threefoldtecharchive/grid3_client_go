// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// DeployerInterface to be used for any deployer
type DeployerInterface interface {
	Deploy(ctx context.Context,
		oldDeploymentIDs map[uint32]uint64,
		newDeployments map[uint32]gridtypes.Deployment,
		newDeploymentsData map[uint32]workloads.DeploymentData,
		newDeploymentSolutionProvider map[uint32]*uint64,
	) (map[uint32]uint64, error)

	Cancel(ctx context.Context,
		oldDeploymentIDs map[uint32]uint64,
		newDeployments map[uint32]gridtypes.Deployment,
	) (map[uint32]uint64, error)

	GetDeployments(ctx context.Context, dls map[uint32]uint64) (map[uint32]gridtypes.Deployment, error)

	Wait(
		ctx context.Context,
		nodeClient *client.NodeClient,
		deploymentID uint64,
		workloadVersions map[string]uint32,
	) error
}

// Deployer to be used for any deployer
type Deployer struct {
	identity        substrate.Identity
	twinID          uint32
	validator       Validator
	ncPool          client.NodeClientGetter
	revertOnFailure bool
	SubstrateConn   subi.SubstrateExt
}

// NewDeployer returns a new deployer
func NewDeployer(
	tfPluginClient TFPluginClient,
	revertOnFailure bool,
) Deployer {

	return Deployer{
		tfPluginClient.Identity,
		tfPluginClient.TwinID,
		&ValidatorImpl{gridClient: tfPluginClient.GridProxyClient},
		tfPluginClient.NcPool,
		revertOnFailure,
		tfPluginClient.SubstrateConn,
	}
}

// Deploy deploys or updates a new deployment given the old deployments' IDs
// TODO: newDeployments should support more than 1 deployment per node ID
func (d *Deployer) Deploy(ctx context.Context,
	oldDeploymentIDs map[uint32]uint64,
	newDeployments map[uint32]gridtypes.Deployment,
	newDeploymentsData map[uint32]workloads.DeploymentData,
	newDeploymentSolutionProvider map[uint32]*uint64,
) (map[uint32]uint64, error) {
	oldDeployments, oldErr := d.GetDeployments(ctx, oldDeploymentIDs)
	if oldErr == nil {
		// check resources only when old deployments are readable
		// being readable means it's a fresh deployment or an update with good nodes
		// this is done to avoid preventing deletion of deployments on dead nodes
		if err := d.validator.Validate(ctx, d.SubstrateConn, oldDeployments, newDeployments); err != nil {
			return oldDeploymentIDs, err
		}
	}

	// ignore oldErr until we need oldDeployments
	currentDeployments, err := d.deploy(ctx, oldDeploymentIDs, newDeployments, newDeploymentsData, newDeploymentSolutionProvider, d.revertOnFailure)

	if err != nil && d.revertOnFailure {
		if oldErr != nil {
			return currentDeployments, fmt.Errorf("failed to deploy deployments: %w; failed to fetch deployment objects to revert deployments: %s; try again", err, oldErr)
		}

		currentDls, rerr := d.deploy(ctx, currentDeployments, oldDeployments, newDeploymentsData, newDeploymentSolutionProvider, false)
		if rerr != nil {
			return currentDls, fmt.Errorf("failed to deploy deployments: %w; failed to revert deployments: %s; try again", err, rerr)
		}
		return currentDls, err
	}

	return currentDeployments, err
}

func (d *Deployer) deploy(
	ctx context.Context,
	oldDeployments map[uint32]uint64,
	newDeployments map[uint32]gridtypes.Deployment,
	newDeploymentsData map[uint32]workloads.DeploymentData,
	newDeploymentSolutionProvider map[uint32]*uint64,
	revertOnFailure bool,
) (currentDeployments map[uint32]uint64, err error) {
	currentDeployments = make(map[uint32]uint64)
	for nodeID, contractID := range oldDeployments {
		currentDeployments[nodeID] = contractID
	}
	// deletions
	/*for node, contractID := range oldDeployments {
		if _, ok := newDeployments[node]; !ok {
			err = d.SubstrateConn.EnsureContractCanceled(d.identity, contractID)
			if err != nil && !strings.Contains(err.Error(), "ContractNotExists") {
				return currentDeployments, errors.Wrap(err, "failed to delete deployment")
			}
			delete(currentDeployments, node)
		}
	}*/

	// creations
	for node, dl := range newDeployments {
		if _, ok := oldDeployments[node]; !ok {
			client, err := d.ncPool.GetNodeClient(d.SubstrateConn, node)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to get node client")
			}

			if err := dl.Sign(d.twinID, d.identity); err != nil {
				return currentDeployments, errors.Wrap(err, "error signing deployment")
			}

			if err := dl.Valid(); err != nil {
				return currentDeployments, errors.Wrap(err, "deployment is invalid")
			}

			hash, err := dl.ChallengeHash()
			log.Printf("[DEBUG] HASH: %#v", hash)

			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to create hash")
			}

			hashHex := hex.EncodeToString(hash)

			publicIPCount, err := CountDeploymentPublicIPs(dl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to count deployment public IPs")
			}
			log.Printf("Number of public ips: %d\n", publicIPCount)

			deploymentDataBytes, err := json.Marshal(newDeploymentsData[node])
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to parse deployment data")
			}

			contractID, err := d.SubstrateConn.CreateNodeContract(d.identity, node, string(deploymentDataBytes), hashHex, publicIPCount, newDeploymentSolutionProvider[node])
			log.Printf("CreateNodeContract returned id: %d\n", contractID)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to create contract")
			}

			dl.ContractID = contractID
			ctx2, cancel := context.WithTimeout(ctx, 4*time.Minute)
			defer cancel()
			err = client.DeploymentDeploy(ctx2, dl)

			if err != nil {
				rerr := d.SubstrateConn.EnsureContractCanceled(d.identity, contractID)
				if rerr != nil {
					return currentDeployments, fmt.Errorf("error sending deployment to the node: %w, error cancelling contract: %s; you must cancel it manually (id: %d)", err, rerr, contractID)
				}
				return currentDeployments, errors.Wrap(err, "error sending deployment to the node")

			}
			currentDeployments[node] = dl.ContractID
			newWorkloadVersions := map[string]uint32{}
			for _, w := range dl.Workloads {
				newWorkloadVersions[w.Name.String()] = 0
			}
			err = d.Wait(ctx, client, dl.ContractID, newWorkloadVersions)

			if err != nil {
				return currentDeployments, errors.Wrap(err, "error waiting deployment")
			}
		}
	}

	// updates
	for node, dl := range newDeployments {
		if oldDeploymentID, ok := oldDeployments[node]; ok {
			newDeploymentHash, err := HashDeployment(dl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get deployment hash")
			}

			client, err := d.ncPool.GetNodeClient(d.SubstrateConn, node)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to get node client")
			}

			oldDl, err := client.DeploymentGet(ctx, oldDeploymentID)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to get old deployment to update it")
			}

			oldDeploymentHash, err := HashDeployment(oldDl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get deployment hash")
			}
			if oldDeploymentHash == newDeploymentHash && SameWorkloadsNames(dl, oldDl) {
				continue
			}

			oldHashes, err := ConstructWorkloadHashes(oldDl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get old workloads hashes")
			}

			newHashes, err := ConstructWorkloadHashes(dl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get new workloads hashes")
			}

			oldWorkloadsVersions := ConstructWorkloadVersions(oldDl)
			newWorkloadsVersions := map[string]uint32{}
			dl.Version = oldDl.Version + 1
			dl.ContractID = oldDl.ContractID
			for idx, w := range dl.Workloads {
				newHash := newHashes[string(w.Name)]
				oldHash, ok := oldHashes[string(w.Name)]
				if !ok || newHash != oldHash {
					dl.Workloads[idx].Version = dl.Version
				} else if ok && newHash == oldHash {
					dl.Workloads[idx].Version = oldWorkloadsVersions[string(w.Name)]
				}
				newWorkloadsVersions[w.Name.String()] = dl.Workloads[idx].Version
			}
			if err := dl.Sign(d.twinID, d.identity); err != nil {
				return currentDeployments, errors.Wrap(err, "error signing deployment")
			}

			if err := dl.Valid(); err != nil {
				return currentDeployments, errors.Wrap(err, "deployment is invalid")
			}

			log.Printf("deployment: %+v", dl)
			hash, err := dl.ChallengeHash()
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to create hash")
			}
			hashHex := hex.EncodeToString(hash)
			log.Printf("[DEBUG] HASH: %s", hashHex)

			// TODO: Destroy and create if publicIPCount is changed
			// publicIPCount, err := countDeploymentPublicIPs(dl)
			contractID, err := d.SubstrateConn.UpdateNodeContract(d.identity, dl.ContractID, "", hashHex)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to update deployment")
			}
			dl.ContractID = contractID
			sub, cancel := context.WithTimeout(ctx, 4*time.Minute)
			defer cancel()
			err = client.DeploymentUpdate(sub, dl)
			if err != nil {
				// cancel previous contract
				return currentDeployments, errors.Wrapf(err, "failed to send deployment update request to node %d", node)
			}
			currentDeployments[node] = dl.ContractID

			err = d.Wait(ctx, client, dl.ContractID, newWorkloadsVersions)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "error waiting deployment")
			}
		}
	}

	return currentDeployments, nil
}

// Cancel cancels an old deployment not given in the new deployments
func (d *Deployer) Cancel(ctx context.Context,
	oldDeploymentIDs map[uint32]uint64,
	newDeployments map[uint32]gridtypes.Deployment,
) (map[uint32]uint64, error) {
	oldDeployments, err := d.GetDeployments(ctx, oldDeploymentIDs)
	if err != nil {
		return oldDeploymentIDs, err
	}

	if err := d.validator.Validate(ctx, d.SubstrateConn, oldDeployments, newDeployments); err != nil {
		return oldDeploymentIDs, err
	}

	currentDeployments := oldDeploymentIDs

	// deletions
	for node, contractID := range oldDeploymentIDs {
		if _, ok := newDeployments[node]; !ok {
			err = d.SubstrateConn.EnsureContractCanceled(d.identity, contractID)
			if err != nil && !strings.Contains(err.Error(), "ContractNotExists") {
				return currentDeployments, errors.Wrap(err, "failed to delete deployment")
			}
			delete(currentDeployments, node)
		}
	}

	return currentDeployments, err
}

// GetDeployments returns deployments from a map of nodes IDs and deployments IDs
func (d *Deployer) GetDeployments(ctx context.Context, dls map[uint32]uint64) (map[uint32]gridtypes.Deployment, error) {
	res := make(map[uint32]gridtypes.Deployment)

	var wg sync.WaitGroup
	var mux = &sync.RWMutex{}
	var resErrors error

	for nodeID, dlID := range dls {

		wg.Add(1)
		go func(nodeID uint32, dlID uint64) {

			defer wg.Done()
			nc, err := d.ncPool.GetNodeClient(d.SubstrateConn, nodeID)
			if err != nil {
				resErrors = multierror.Append(resErrors, errors.Wrapf(err, "failed to get a client for node %d", nodeID))
				return
			}

			sub, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			dl, err := nc.DeploymentGet(sub, dlID)
			if err != nil {
				resErrors = multierror.Append(resErrors, errors.Wrapf(err, "failed to get deployment %d of node %d", dlID, nodeID))
				return
			}

			mux.Lock()
			res[nodeID] = dl
			mux.Unlock()

		}(nodeID, dlID)
	}

	wg.Wait()
	if resErrors != nil {
		return nil, resErrors
	}
	return res, nil
}

// Progress struct for checking progress
type Progress struct {
	time    time.Time
	stateOk int
}

func getExponentialBackoff(initialInterval time.Duration, multiplier float64, maxInterval time.Duration, maxElapsedTime time.Duration) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = initialInterval
	b.Multiplier = multiplier
	b.MaxInterval = maxInterval
	b.MaxElapsedTime = maxElapsedTime
	return b
}

// Wait waits for a deployment to be deployed on node
func (d *Deployer) Wait(
	ctx context.Context,
	nodeClient *client.NodeClient,
	deploymentID uint64,
	workloadVersions map[string]uint32,
) error {
	lastProgress := Progress{time.Now(), 0}
	numberOfWorkloads := len(workloadVersions)

	deploymentError := backoff.Retry(func() error {
		stateOk := 0
		sub, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		deploymentChanges, err := nodeClient.DeploymentChanges(sub, deploymentID)
		if err != nil {
			return backoff.Permanent(err)
		}

		for _, wl := range deploymentChanges {
			if _, ok := workloadVersions[wl.Name.String()]; ok && wl.Version == workloadVersions[wl.Name.String()] {
				var errString string
				switch wl.Result.State {
				case gridtypes.StateOk:
					stateOk++
				case gridtypes.StateError:
					errString = fmt.Sprintf("workload %s within deployment %d failed with error: %s", wl.Name, deploymentID, wl.Result.Error)
				case gridtypes.StateDeleted:
					errString = fmt.Sprintf("workload %s state within deployment %d is deleted: %s", wl.Name, deploymentID, wl.Result.Error)
				case gridtypes.StatePaused:
					errString = fmt.Sprintf("workload %s state within deployment %d is paused: %s", wl.Name, deploymentID, wl.Result.Error)
				case gridtypes.StateUnChanged:
					errString = fmt.Sprintf("workload %s within deployment %d was not updated: %s", wl.Name, deploymentID, wl.Result.Error)
				}
				if errString != "" {
					return backoff.Permanent(errors.New(errString))
				}
			}
		}

		if stateOk == numberOfWorkloads {
			return nil
		}

		currentProgress := Progress{time.Now(), stateOk}
		if lastProgress.stateOk < currentProgress.stateOk {
			lastProgress = currentProgress
		} else if currentProgress.time.Sub(lastProgress.time) > 4*time.Minute {
			timeoutError := fmt.Errorf("waiting for deployment %d timed out", deploymentID)
			return backoff.Permanent(timeoutError)
		}

		return errors.New("deployment in progress")
	},
		backoff.WithContext(getExponentialBackoff(3*time.Second, 1.25, 40*time.Second, 50*time.Minute), ctx))

	return deploymentError
}
