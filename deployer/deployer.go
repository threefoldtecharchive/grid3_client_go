// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	proxyTypes "github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// MockDeployer to be used for any deployer in mock testing
type MockDeployer interface { //TODO: Change Name && separate them
	Deploy(ctx context.Context,
		oldDeploymentIDs map[uint32]uint64,
		newDeployments map[uint32]gridtypes.Deployment,
		newDeploymentSolutionProvider map[uint32]*uint64,
	) (map[uint32]uint64, error)

	Cancel(ctx context.Context,
		contractID uint64,
	) error

	GetDeployments(ctx context.Context, dls map[uint32]uint64) (map[uint32]gridtypes.Deployment, error)
}

// Deployer to be used for any deployer
type Deployer struct {
	identity        substrate.Identity
	twinID          uint32
	gridProxyClient proxy.Client
	ncPool          client.NodeClientGetter
	revertOnFailure bool
	substrateConn   subi.SubstrateExt
}

// NewDeployer returns a new deployer
func NewDeployer(
	tfPluginClient TFPluginClient,
	revertOnFailure bool,
) Deployer {

	return Deployer{
		tfPluginClient.Identity,
		tfPluginClient.twinID,
		tfPluginClient.GridProxyClient,
		tfPluginClient.NcPool,
		revertOnFailure,
		tfPluginClient.SubstrateConn,
	}
}

// Deploy deploys or updates a new deployment given the old deployments' IDs
func (d *Deployer) Deploy(ctx context.Context,
	oldDeploymentIDs map[uint32]uint64,
	newDeployments map[uint32]gridtypes.Deployment,
	newDeploymentSolutionProvider map[uint32]*uint64,
) (map[uint32]uint64, error) {
	oldDeployments, oldErr := d.GetDeployments(ctx, oldDeploymentIDs)
	if oldErr == nil {
		// check resources only when old deployments are readable
		// being readable means it's a fresh deployment or an update with good nodes
		// this is done to avoid preventing deletion of deployments on dead nodes
		if err := d.Validate(ctx, oldDeployments, newDeployments); err != nil {
			return oldDeploymentIDs, err
		}
	}

	// ignore oldErr until we need oldDeployments
	currentDeployments, err := d.deploy(ctx, oldDeploymentIDs, newDeployments, newDeploymentSolutionProvider, d.revertOnFailure)

	if err != nil && d.revertOnFailure {
		if oldErr != nil {
			return currentDeployments, fmt.Errorf("failed to deploy deployments: %w; failed to fetch deployment objects to revert deployments: %s; try again", err, oldErr)
		}

		currentDls, rerr := d.deploy(ctx, currentDeployments, oldDeployments, newDeploymentSolutionProvider, false)
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
			client, err := d.ncPool.GetNodeClient(d.substrateConn, node)
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

			contractID, err := d.substrateConn.CreateNodeContract(d.identity, node, dl.Metadata, hashHex, publicIPCount, newDeploymentSolutionProvider[node])
			log.Printf("CreateNodeContract returned id: %d\n", contractID)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "failed to create contract")
			}

			dl.ContractID = contractID
			ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
			defer cancel()
			err = client.DeploymentDeploy(ctx, dl)

			if err != nil {
				rerr := d.substrateConn.EnsureContractCanceled(d.identity, contractID)
				if rerr != nil {
					return currentDeployments, fmt.Errorf("error sending deployment to the node: %w, error cancelling contract: %s; you must cancel it manually (id: %d)", err, rerr, contractID)
				}
				return currentDeployments, errors.Wrap(err, "error sending deployment to the node")

			}
			currentDeployments[node] = dl.ContractID
			newWorkloadVersions := make(map[string]uint32)
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

			client, err := d.ncPool.GetNodeClient(d.substrateConn, node)
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

			oldHashes, err := GetWorkloadHashes(oldDl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get old workloads hashes")
			}

			newHashes, err := GetWorkloadHashes(dl)
			if err != nil {
				return currentDeployments, errors.Wrap(err, "couldn't get new workloads hashes")
			}

			oldWorkloadsVersions := ConstructWorkloadVersions(oldDl)
			newWorkloadsVersions := make(map[string]uint32)
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
			contractID, err := d.substrateConn.UpdateNodeContract(d.identity, dl.ContractID, "", hashHex)
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
	contractID uint64,
) error {

	err := d.substrateConn.EnsureContractCanceled(d.identity, contractID)
	if err != nil {
		return errors.Wrapf(err, "failed to delete deployment: %d", contractID)
	}

	return nil
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
			nc, err := d.ncPool.GetNodeClient(d.substrateConn, nodeID)
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

// Validate is a best effort validation. it returns an error if it's very sure there's a problem
//   - validates old deployments nodes (for update cases) and new deployments nodes
//   - validates nodes' farm
//   - checks free public ips
//   - checks free nodes capacity
//   - checks PublicConfig Ipv4 for fqdn gateway
//   - checks PublicConfig domain for name gateway
//
// errors that may arise because of dead nodes are ignored.
// if a real error dodges the validation, it'll be fail anyway in the deploying phase
func (d *Deployer) Validate(ctx context.Context, oldDeployments map[uint32]gridtypes.Deployment, newDeployments map[uint32]gridtypes.Deployment) error {
	farmIPs := make(map[int]int)
	nodeMap := make(map[uint32]proxyTypes.NodeWithNestedCapacity)

	for node := range oldDeployments {
		nodeInfo, err := d.gridProxyClient.Node(node)
		if err != nil {
			return errors.Wrapf(err, "couldn't get node %d data from the grid proxy", node)
		}
		nodeMap[node] = nodeInfo
		farmIPs[nodeInfo.FarmID] = 0
	}

	for node := range newDeployments {
		if _, ok := nodeMap[node]; ok {
			continue
		}
		nodeInfo, err := d.gridProxyClient.Node(node)
		if err != nil {
			return errors.Wrapf(err, "couldn't get node %d data from the grid proxy", node)
		}
		nodeMap[node] = nodeInfo
		farmIPs[nodeInfo.FarmID] = 0
	}

	for farm := range farmIPs {
		farmUint64 := uint64(farm)
		farmInfo, _, err := d.gridProxyClient.Farms(proxyTypes.FarmFilter{
			FarmID: &farmUint64,
		}, proxyTypes.Limit{
			Page: 1,
			Size: 1,
		})
		if err != nil {
			return errors.Wrapf(err, "couldn't get farm %d data from the grid proxy", farm)
		}
		if len(farmInfo) == 0 {
			return fmt.Errorf("farm %d not returned from the proxy", farm)
		}
		for _, ip := range farmInfo[0].PublicIps {
			if ip.ContractID == 0 {
				farmIPs[farm]++
			}
		}
	}

	for node, dl := range oldDeployments {
		nodeData, ok := nodeMap[node]
		if !ok {
			return fmt.Errorf("node %d not returned from the grid proxy", node)
		}

		publicIPCount, err := CountDeploymentPublicIPs(dl)
		if err != nil {
			return errors.Wrap(err, "failed to count deployment public IPs")
		}

		farmIPs[nodeData.FarmID] += int(publicIPCount)
	}

	for node, dl := range newDeployments {
		oldDl, alreadyExists := oldDeployments[node]
		if err := dl.Valid(); err != nil {
			return errors.Wrap(err, "invalid deployment")
		}

		needed, err := Capacity(dl)
		if err != nil {
			return err
		}

		publicIPCount, err := CountDeploymentPublicIPs(dl)
		if err != nil {
			return errors.Wrap(err, "failed to count deployment public IPs")
		}
		requiredIPs := int(publicIPCount)
		nodeInfo := nodeMap[node]
		if alreadyExists {
			oldCap, err := Capacity(oldDl)
			if err != nil {
				return errors.Wrapf(err, "couldn't read old deployment %d of node %d capacity", oldDl.ContractID, node)
			}
			addCapacity(&nodeInfo.Capacity.Total, &oldCap)
			contract, err := d.substrateConn.GetContract(oldDl.ContractID)
			if err != nil {
				return errors.Wrapf(err, "couldn't get node contract %d", oldDl.ContractID)
			}
			current := int(contract.PublicIPCount())
			if requiredIPs > current {
				return fmt.Errorf(
					"currently, it's not possible to increase the number of reserved public ips in a deployment, node: %d, current: %d, requested: %d",
					node,
					current,
					requiredIPs,
				)
			}
		}

		farmIPs[nodeInfo.FarmID] -= requiredIPs
		if farmIPs[nodeInfo.FarmID] < 0 {
			return fmt.Errorf("farm %d doesn't have enough public ips", nodeInfo.FarmID)
		}
		if HasWorkload(&dl, zos.GatewayFQDNProxyType) && nodeInfo.PublicConfig.Ipv4 == "" {
			return fmt.Errorf("node %d can't deploy a fqdn workload as it doesn't have a public ipv4 configured", node)
		}
		if HasWorkload(&dl, zos.GatewayNameProxyType) && nodeInfo.PublicConfig.Domain == "" {
			return fmt.Errorf("node %d can't deploy a gateway name workload as it doesn't have a domain configured", node)
		}
		mru := nodeInfo.Capacity.Total.MRU - nodeInfo.Capacity.Used.MRU
		hru := nodeInfo.Capacity.Total.HRU - nodeInfo.Capacity.Used.HRU
		sru := 2*nodeInfo.Capacity.Total.SRU - nodeInfo.Capacity.Used.SRU
		if mru < needed.MRU ||
			sru < needed.SRU ||
			hru < needed.HRU {
			free := gridtypes.Capacity{
				HRU: hru,
				MRU: mru,
				SRU: sru,
			}
			return fmt.Errorf("node %d doesn't have enough resources. needed: %v, free: %v", node, capacityPrettyPrint(needed), capacityPrettyPrint(free))
		}
	}
	return nil
}

// capacityPrettyPrint prints the capacity data
func capacityPrettyPrint(cap gridtypes.Capacity) string {
	return fmt.Sprintf("[mru: %d, sru: %d, hru: %d]", cap.MRU, cap.SRU, cap.HRU)
}

// addCapacity adds a new data for capacity
func addCapacity(cap *proxyTypes.Capacity, add *gridtypes.Capacity) {
	cap.CRU += add.CRU
	cap.MRU += add.MRU
	cap.SRU += add.SRU
	cap.HRU += add.HRU
}
