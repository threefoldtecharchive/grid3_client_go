// Package deployer is grid deployer
package deployer

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// GatewayFQDNDeployer for deploying a GatewayFqdn
type GatewayFQDNDeployer struct {
	Gw               workloads.GatewayFQDNProxy
	ID               string
	Description      string
	Node             uint32
	NodeDeploymentID map[uint32]uint64

	ncPool         client.NodeClientGetter
	TFPluginClient *TFPluginClient
	deployer       Deployer
}

// Generates new gateway fqdn deployer
func NewGatewayFqdnDeployer(tfPluginClient *TFPluginClient) GatewayFQDNDeployer {
	gatewayFQDN := GatewayFQDNDeployer{
		ncPool:   client.NewNodeClientPool(tfPluginClient.RMB),
		deployer: Deployer{},
	}

	gatewayFQDN.TFPluginClient = tfPluginClient
	gatewayFQDN.deployer = NewDeployer(*tfPluginClient, true)
	return gatewayFQDN
}

// Validate validates gatewayFdqn deployer
func (k *GatewayFQDNDeployer) Validate(ctx context.Context) error {
	sub := k.TFPluginClient.SubstrateConn
	if err := validateAccountBalanceForExtrinsics(sub, k.TFPluginClient.Identity); err != nil {
		return err
	}
	return client.AreNodesUp(ctx, sub, []uint32{k.Node}, k.ncPool)
}

// GenerateVersionlessDeploymentsAndWorkloads generates deployments for gatewayFqdn deployer without versions
func (k *GatewayFQDNDeployer) GenerateVersionlessDeployments(ctx context.Context, gw *workloads.GatewayFQDNProxy) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	var wls []gridtypes.Workload

	err := k.Validate(ctx)
	if err != nil {
		return nil, err
	}

	dl := workloads.NewGridDeployment(k.deployer.twinID, wls)
	wls = append(wls, gw.ZosWorkload())
	dl.Workloads = wls
	deployments[k.Node] = dl

	return deployments, nil
}

// Deploy deploys the GatewayFQDN deployments using the deployer
func (k *GatewayFQDNDeployer) Deploy(ctx context.Context, gw *workloads.GatewayFQDNProxy) error {
	sub := k.TFPluginClient.SubstrateConn
	if err := k.Validate(ctx); err != nil {
		return err
	}
	newDeployments, err := k.GenerateVersionlessDeployments(ctx, gw)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	deploymentData := DeploymentData{
		Name:        gw.Name,
		Type:        gw.FQDN,
		ProjectName: gw.ZosWorkload().Description,
	}

	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	newDeploymentsData[k.Node] = deploymentData
	newDeploymentsSolutionProvider[k.Node] = nil

	k.NodeDeploymentID, err = k.deployer.Deploy(ctx, k.NodeDeploymentID, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)
	if k.ID == "" && k.NodeDeploymentID[k.Node] != 0 {
		k.ID = strconv.FormatUint(k.NodeDeploymentID[k.Node], 10)
	}
	return err
}

func (k *GatewayFQDNDeployer) syncContracts(ctx context.Context, sub subi.SubstrateExt) (err error) {
	if err := sub.DeleteInvalidContracts(k.NodeDeploymentID); err != nil {
		return err
	}
	if len(k.NodeDeploymentID) == 0 {
		// delete resource in case nothing is active (reflects only on read)
		k.ID = ""
	}
	return nil
}

func (k *GatewayFQDNDeployer) sync(ctx context.Context, sub subi.SubstrateExt) error {
	if err := k.syncContracts(ctx, sub); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}

	dls, err := k.deployer.GetDeployments(ctx, k.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}
	dl := dls[k.Node]
	wl, _ := dl.Get(gridtypes.Name(k.Gw.Name))
	k.Gw = workloads.GatewayFQDNProxy{}
	if wl != nil && wl.Result.State.IsOkay() {
		k.Gw, err = workloads.NewGatewayFQDNProxyFromZosWorkload(*wl.Workload)
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *GatewayFQDNDeployer) Cancel(ctx context.Context) (err error) {
	newDeployments := make(map[uint32]gridtypes.Deployment)
	newDeploymentsData := make(map[uint32]workloads.DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	k.NodeDeploymentID, err = k.deployer.Deploy(ctx, sub, k.NodeDeploymentID, newDeployments, newDeploymentsData, newDeploymentsSolutionProvider)

	return err
}
