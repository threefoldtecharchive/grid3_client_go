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

// NewGatewayFqdnDeployer generates new gateway fqdn deployer
func NewGatewayFqdnDeployer(tfPluginClient *TFPluginClient) GatewayFQDNDeployer {
	deployer := NewDeployer(*tfPluginClient, true)
	gatewayFQDN := GatewayFQDNDeployer{
		tfPluginClient: tfPluginClient,
		deployer:       &deployer,
	}

	return gatewayFQDN
}

// Validate validates gateway FQDN deployer
func (d *GatewayFQDNDeployer) Validate(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	sub := d.tfPluginClient.SubstrateConn
	if len(gw.Name) == 0 {
		return errors.New("gateway workload must have a name")
	}
	if err := validateAccountBalanceForExtrinsics(sub, d.tfPluginClient.identity); err != nil {
		return err
	}
	return client.AreNodesUp(ctx, sub, []uint32{gw.NodeID}, d.tfPluginClient.NcPool)
}

// GenerateVersionlessDeployments generates deployments for gatewayFqdn deployer without versions
func (d *GatewayFQDNDeployer) GenerateVersionlessDeployments(ctx context.Context, gw *workloads.GatewayFQDNProxy) (map[uint32]gridtypes.Deployment, error) {
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

// Deploy deploys the GatewayFQDN deployments using the deployer
func (d *GatewayFQDNDeployer) Deploy(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	if err := d.Validate(ctx, gw); err != nil {
		return err
	}

	newDeployments, err := d.GenerateVersionlessDeployments(ctx, gw)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	// TODO: solution providers
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	newDeploymentsSolutionProvider[gw.NodeID] = nil

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment
	currentDeployments, err := d.deployer.Deploy(ctx, oldDeployments, newDeployments, newDeploymentsSolutionProvider)

	// update state
	if currentDeployments[gw.NodeID] != 0 {
		if gw.NodeDeploymentID == nil {
			gw.NodeDeploymentID = make(map[uint32]uint64)
		}

		gw.ContractID = currentDeployments[gw.NodeID]
		gw.NodeDeploymentID[gw.NodeID] = gw.ContractID
		d.tfPluginClient.StateLoader.currentNodeDeployment[gw.NodeID] = gw.ContractID
	}

	return err
}

// Cancel cancels a gateway deployment
func (d *GatewayFQDNDeployer) Cancel(ctx context.Context, gw *workloads.GatewayFQDNProxy) (err error) {
	if err := d.Validate(ctx, gw); err != nil {
		return err
	}

	oldDeployments := d.tfPluginClient.StateLoader.currentNodeDeployment

	err = d.deployer.Cancel(ctx, oldDeployments[gw.NodeID])
	if err != nil {
		return err
	}

	// update state
	gw.ContractID = 0
	delete(gw.NodeDeploymentID, gw.NodeID)
	delete(d.tfPluginClient.StateLoader.currentNodeDeployment, gw.NodeID)

	return nil
}

// TODO: check sync added or not ??
func (d *GatewayFQDNDeployer) syncContracts(ctx context.Context, gw *workloads.GatewayFQDNProxy) (err error) {
	if err := d.tfPluginClient.SubstrateConn.DeleteInvalidContracts(gw.NodeDeploymentID); err != nil {
		return err
	}
	if len(gw.NodeDeploymentID) == 0 {
		gw.ContractID = 0
	}
	return nil
}

// Sync syncs the gateway deployments
func (d *GatewayFQDNDeployer) Sync(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	if err := d.syncContracts(ctx, gw); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}

	dls, err := d.deployer.GetDeployments(ctx, gw.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}

	dl := dls[gw.NodeID]
	wl, _ := dl.Get(gridtypes.Name(gw.Name))

	gwWorkload := workloads.GatewayFQDNProxy{}
	gw.Backends = gwWorkload.Backends
	gw.Name = gwWorkload.Name
	gw.FQDN = gwWorkload.FQDN
	gw.TLSPassthrough = gwWorkload.TLSPassthrough

	if wl != nil && wl.Result.State.IsOkay() {
		gwWorkload, err := workloads.NewGatewayFQDNProxyFromZosWorkload(*wl.Workload)
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
