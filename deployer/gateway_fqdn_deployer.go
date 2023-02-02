// Package deployer is grid deployer
package deployer

import (
	"context"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// GatewayFQDNDeployer for deploying a GatewayFqdn
type GatewayFQDNDeployer struct {
	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// Generates new gateway fqdn deployer
func NewGatewayFqdnDeployer(tfPluginClient *TFPluginClient) GatewayFQDNDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	gatewayFQDN := GatewayFQDNDeployer{
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}

	return gatewayFQDN
}

// Validate validates gatewayFdqn deployer
func (k *GatewayFQDNDeployer) Validate(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	sub := k.tfPluginClient.SubstrateConn
	if err := validateAccountBalanceForExtrinsics(sub, k.tfPluginClient.Identity); err != nil {
		return err
	}
	return client.AreNodesUp(ctx, sub, []uint32{gw.NodeID}, k.tfPluginClient.NcPool)
}

// GenerateVersionlessDeployments generates deployments for gatewayFqdn deployer without versions
func (k *GatewayFQDNDeployer) GenerateVersionlessDeployments(ctx context.Context, gw *workloads.GatewayFQDNProxy) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)

	dl := workloads.NewGridDeployment(k.tfPluginClient.TwinID, []gridtypes.Workload{})
	dl.Workloads = append(dl.Workloads, gw.ZosWorkload())

	deployments[gw.NodeID] = dl
	return deployments, nil
}

// Deploy deploys the GatewayFQDN deployments using the deployer
func (k *GatewayFQDNDeployer) Deploy(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	if err := k.Validate(ctx, gw); err != nil {
		return err
	}

	newDeployments, err := k.GenerateVersionlessDeployments(ctx, gw)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	if len(gw.SolutionType) == 0 {
		gw.SolutionType = "Gateway"
	}

	deploymentData := workloads.DeploymentData{
		Name:        gw.FQDN,
		Type:        "Gateway Fqdn",
		ProjectName: gw.SolutionType,
	}

	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	newDeploymentsData[gw.NodeID] = deploymentData
	// TODO: solution providers
	newDeploymentsSolutionProvider[gw.NodeID] = nil

	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment
	gw.NodeDeploymentID, err = k.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

	// update state
	if gw.ContractID == 0 && gw.NodeDeploymentID[gw.NodeID] != 0 {
		gw.ContractID = gw.NodeDeploymentID[gw.NodeID]
	}
	k.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID] = gw.ContractID

	return err
}

func (k *GatewayFQDNDeployer) Cancel(ctx context.Context, gw *workloads.GatewayFQDNProxy) (err error) {
	if err := k.Validate(ctx, gw); err != nil {
		return err
	}

	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment

	// construct new deployments to have all old deployments except the given one
	newDeployments := make(map[uint32]gridtypes.Deployment)
	for nodeID := range oldDeployments {
		if gw.NodeID != nodeID {
			newDeployments[nodeID] = gridtypes.Deployment{}
		}
	}

	gw.NodeDeploymentID, err = k.deployer.Cancel(ctx, oldDeployments, newDeployments)
	return err
}

func (k *GatewayFQDNDeployer) syncContracts(ctx context.Context, gw *workloads.GatewayFQDNProxy) (err error) {
	if err := k.tfPluginClient.SubstrateConn.DeleteInvalidContracts(gw.NodeDeploymentID); err != nil {
		return err
	}
	if len(gw.NodeDeploymentID) == 0 {
		// delete resource in case nothing is active (reflects only on read)
		gw.ContractID = 0
	}
	return nil
}

// Sync syncs the gateway deployments
func (k *GatewayFQDNDeployer) Sync(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	if err := k.syncContracts(ctx, gw); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}

	dls, err := k.deployer.GetDeployments(ctx, gw.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}

	dl := dls[gw.NodeID]
	wl, _ := dl.Get(gridtypes.Name(gw.Name))

	if wl != nil && wl.Result.State.IsOkay() {
		gwWl, err := workloads.NewGatewayFQDNProxyFromZosWorkload(*wl.Workload)
		gw = &gwWl
		if err != nil {
			return err
		}
	}

	return nil
}
