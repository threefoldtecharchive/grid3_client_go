// Package deployer is grid deployer
package deployer

import (
	"context"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// GatewayNameDeployer for deploying a GatewayName
type GatewayNameDeployer struct {
	tfPluginClient *TFPluginClient
	deployer       DeployerInterface
}

// NewGatewayNameDeployer generates new gateway name deployer
func NewGatewayNameDeployer(tfPluginClient *TFPluginClient) GatewayNameDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	gatewayName := GatewayNameDeployer{
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}

	return gatewayName
}

// Validate validates gatewayName deployer
func (k *GatewayNameDeployer) Validate(ctx context.Context, gw *workloads.GatewayNameProxy) error {
	sub := k.tfPluginClient.SubstrateConn
	if err := validateAccountBalanceForExtrinsics(sub, k.tfPluginClient.Identity); err != nil {
		return err
	}
	return client.AreNodesUp(ctx, sub, []uint32{gw.NodeID}, k.tfPluginClient.NcPool)
}

// GenerateVersionlessDeployments generates deployments for gateway name deployer without versions
func (k *GatewayNameDeployer) GenerateVersionlessDeployments(ctx context.Context, gw *workloads.GatewayNameProxy) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)

	dl := workloads.NewGridDeployment(k.tfPluginClient.TwinID, []gridtypes.Workload{})
	dl.Workloads = append(dl.Workloads, gw.ZosWorkload())

	deployments[gw.NodeID] = dl
	return deployments, nil
}

// InvalidateNameContract invalidates name contract
func (k *GatewayNameDeployer) InvalidateNameContract(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if gw.NameContractID == 0 {
		return
	}

	gw.NameContractID, err = k.tfPluginClient.SubstrateConn.InvalidateNameContract(
		ctx,
		k.tfPluginClient.Identity,
		gw.NameContractID,
		gw.Name,
	)
	return
}

// Deploy deploys the GatewayName deployments using the deployer
func (k *GatewayNameDeployer) Deploy(ctx context.Context, gw *workloads.GatewayNameProxy) error {
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
		Name:        gw.Name,
		Type:        "Gateway Name",
		ProjectName: gw.SolutionType,
	}
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	newDeploymentsData[gw.NodeID] = deploymentData
	newDeploymentsSolutionProvider[gw.NodeID] = nil

	if err := k.InvalidateNameContract(ctx, gw); err != nil {
		return err
	}
	if gw.NameContractID == 0 {
		gw.NameContractID, err = k.tfPluginClient.SubstrateConn.CreateNameContract(k.tfPluginClient.Identity, gw.Name)
		if err != nil {
			return err
		}
	}
	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment

	currentDeployments, err := k.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

	// update state
	gw.ContractID = currentDeployments[gw.NodeID]
	gw.NodeDeploymentID = currentDeployments
	k.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID] = gw.ContractID

	return err
}

func (k *GatewayNameDeployer) syncContracts(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if err := k.tfPluginClient.SubstrateConn.DeleteInvalidContracts(gw.NodeDeploymentID); err != nil {
		return err
	}
	valid, err := k.tfPluginClient.SubstrateConn.IsValidContract(gw.NameContractID)
	if err != nil {
		return err
	}
	if !valid {
		gw.NameContractID = 0
	}
	if gw.NameContractID == 0 && len(gw.NodeDeploymentID) == 0 {
		// delete resource in case nothing is active (reflects only on read)
		gw.ContractID = 0
	}
	return nil
}

// Sync syncs the gateway deployments
func (k *GatewayNameDeployer) sync(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if err := k.syncContracts(ctx, gw); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}
	dls, err := k.deployer.GetDeployments(ctx, gw.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}
	dl := dls[gw.NodeID]
	wl, _ := dl.Get(gridtypes.Name(gw.Name))

	gwWorkload := workloads.GatewayNameProxy{}
	gw.Backends = gwWorkload.Backends
	gw.Name = gwWorkload.Name
	gw.FQDN = gwWorkload.FQDN
	gw.TLSPassthrough = gwWorkload.TLSPassthrough
	// if the node acknowledges it, we are golden
	if wl != nil && wl.Result.State.IsOkay() {
		gwWorkload, err := workloads.NewGatewayNameProxyFromZosWorkload(*wl.Workload)
		gw.Backends = gwWorkload.Backends
		gw.Name = gwWorkload.Name
		gw.FQDN = gwWorkload.FQDN
		gw.TLSPassthrough = gwWorkload.TLSPassthrough
		if err != nil {
			return err
		}
	}
	return nil
}

// Cancel cancels the gatewayName deployment
func (k *GatewayNameDeployer) Cancel(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	oldDeployments := k.tfPluginClient.StateLoader.currentNodeDeployment

	err = k.deployer.Cancel(ctx, oldDeployments[gw.NodeID])

	if err != nil {
		return err
	}

	gw.ContractID = 0
	delete(k.tfPluginClient.StateLoader.currentNodeDeployment, gw.NodeID)
	delete(gw.NodeDeploymentID, gw.NodeID)

	if gw.NameContractID != 0 {
		if err := k.tfPluginClient.SubstrateConn.EnsureContractCanceled(k.tfPluginClient.Identity, gw.NameContractID); err != nil {
			return err
		}
		gw.NameContractID = 0
	}
	return nil
}
