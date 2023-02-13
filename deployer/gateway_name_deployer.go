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
	deployer       MockDeployer
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
func (d *GatewayNameDeployer) Validate(ctx context.Context, gw *workloads.GatewayNameProxy) error {
	sub := d.tfPluginClient.SubstrateConn
	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.identity); err != nil {
		return err
	}
	return client.AreNodesUp(ctx, sub, []uint32{gw.NodeID}, d.tfPluginClient.NcPool)
}

// GenerateVersionlessDeployments generates deployments for gateway name deployer without versions
func (d *GatewayNameDeployer) GenerateVersionlessDeployments(ctx context.Context, gw *workloads.GatewayNameProxy) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	var err error

	dl := workloads.NewGridDeployment(d.tfPluginClient.twinID, []gridtypes.Workload{})
	dl.Workloads = append(dl.Workloads, gw.ZosWorkload())

	dl.Metadata, err = gw.GenerateMetadata()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate gateway FQDN deployment %s metadata", gw.Name)
	}

	deployments[gw.NodeID] = dl
	return deployments, nil
}

// Deploy deploys the GatewayName deployments using the deployer
func (d *GatewayNameDeployer) Deploy(ctx context.Context, gw *workloads.GatewayNameProxy) error {
	if err := d.Validate(ctx, gw); err != nil {
		return err
	}
	newDeployments, err := d.GenerateVersionlessDeployments(ctx, gw)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	newDeploymentsSolutionProvider[gw.NodeID] = nil

	if err := d.InvalidateNameContract(ctx, gw); err != nil {
		return err
	}
	if gw.NameContractID == 0 {
		gw.NameContractID, err = d.tfPluginClient.SubstrateConn.CreateNameContract(d.tfPluginClient.identity, gw.Name)
		if err != nil {
			return err
		}
	}

	gw.NodeDeploymentID, err = d.deployer.Deploy(ctx, gw.NodeDeploymentID, newDeployments, newDeploymentsSolutionProvider)

	// update state
	if gw.NodeDeploymentID[gw.NodeID] != 0 {
		gw.ContractID = gw.NodeDeploymentID[gw.NodeID]
		if !workloads.Contains(d.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID], gw.ContractID) {
			d.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID] = append(d.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID], gw.ContractID)
		}
	}

	return err
}

// Cancel cancels the gatewayName deployment
func (d *GatewayNameDeployer) Cancel(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if err := d.Validate(ctx, gw); err != nil {
		return err
	}

	err = d.deployer.Cancel(ctx, gw.NodeDeploymentID[gw.NodeID])
	if err != nil {
		return err
	}

	gw.ContractID = 0
	delete(gw.NodeDeploymentID, gw.NodeID)
	delete(d.tfPluginClient.StateLoader.currentNodeDeployment, gw.NodeID)

	if gw.NameContractID != 0 {
		if err := d.tfPluginClient.SubstrateConn.EnsureContractCanceled(d.tfPluginClient.identity, gw.NameContractID); err != nil {
			return err
		}
		gw.NameContractID = 0
	}

	return nil
}

// InvalidateNameContract invalidates name contract
func (d *GatewayNameDeployer) InvalidateNameContract(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if gw.NameContractID == 0 {
		return
	}

	gw.NameContractID, err = d.tfPluginClient.SubstrateConn.InvalidateNameContract(
		ctx,
		d.tfPluginClient.identity,
		gw.NameContractID,
		gw.Name,
	)
	return
}

func (d *GatewayNameDeployer) syncContracts(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if err := d.tfPluginClient.SubstrateConn.DeleteInvalidContracts(gw.NodeDeploymentID); err != nil {
		return err
	}
	valid, err := d.tfPluginClient.SubstrateConn.IsValidContract(gw.NameContractID)
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
func (d *GatewayNameDeployer) Sync(ctx context.Context, gw *workloads.GatewayNameProxy) (err error) {
	if err := d.syncContracts(ctx, gw); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}
	dls, err := d.deployer.GetDeployments(ctx, gw.NodeDeploymentID)
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
