package deployer

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

type GatewayNameDeployer struct {
	Gw               workloads.GatewayNameProxy
	ID               string
	Node             uint32
	Description      string
	NodeDeploymentID map[uint32]uint64
	NameContractID   uint64

	TFPluginClient *TFPluginClient
	ncPool         client.NodeClientGetter
	deployer       Deployer
}

// Generates new gateway name deployer
func NewGatewayNameDeployer(tfPluginClient *TFPluginClient) GatewayNameDeployer {
	gatewayName := GatewayNameDeployer{
		ncPool: client.NewNodeClientPool(tfPluginClient.RMB),
		deployer: Deployer{},
	}

	gatewayName.TFPluginClient = tfPluginClient
	gatewayName.deployer = NewDeployer(*tfPluginClient, true)
	return gatewayName
}

// Validate validates gatewayName deployer
func (k *GatewayNameDeployer) Validate(ctx context.Context, sub subi.SubstrateExt) error {
	return client.AreNodesUp(ctx, sub, []uint32{k.Node}, k.ncPool)
}

// GenerateVersionlessDeploymentsAndWorkloads generates deployments for gateway name deployer without versions
func (k *GatewayNameDeployer) GenerateVersionlessDeployments(ctx context.Context,sub subi.SubstrateExt) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	var wls []gridtypes.Workload

	err := k.Validate(ctx,sub)
	if err != nil {
		return nil, err
	}

	dl := workloads.NewGridDeployment(k.deployer.twinID, wls)
	wls = append(wls, k.Gw.ZosWorkload())
	dl.Workloads = wls
	deployments[k.Node] = dl

	return deployments, nil
}

func (k *GatewayNameDeployer) InvalidateNameContract(ctx context.Context, sub subi.SubstrateExt) (err error) {
	if k.NameContractID == 0 {
		return
	}

	k.NameContractID, err = sub.InvalidateNameContract(
		ctx,
		k.TFPluginClient.Identity,
		k.NameContractID,
		k.Gw.Name,
	)
	return
}

func (k *GatewayNameDeployer) Deploy(ctx context.Context, sub subi.SubstrateExt) error {
	if err := k.Validate(ctx, sub); err != nil {
		return err
	}
	newDeployments, err := k.GenerateVersionlessDeployments(ctx,sub)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}

	deploymentData := DeploymentData{
		Name: k.Gw.Name,
		Type: "Gateway Name",
	}
	newDeploymentsData := make(map[uint32]DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)

	newDeploymentsData[k.Node] = deploymentData
	newDeploymentsSolutionProvider[k.Node] = nil

	if err := k.InvalidateNameContract(ctx, sub); err != nil {
		return err
	}
	if k.NameContractID == 0 {
		k.NameContractID, err = sub.CreateNameContract(k.TFPluginClient.Identity, k.Gw.Name)
		if err != nil {
			return err
		}
	}
	if k.ID == "" {
		// create the resource if the contract is created
		k.ID = uuid.New().String()
	}
	k.NodeDeploymentID, err = k.deployer.Deploy(ctx, sub, k.NodeDeploymentID, newDeployments,newDeploymentsData, newDeploymentsSolutionProvider)
	return err
}

func (k *GatewayNameDeployer) syncContracts(ctx context.Context, sub subi.SubstrateExt) (err error) {
	if err := sub.DeleteInvalidContracts(k.NodeDeploymentID); err != nil {
		return err
	}
	valid, err := sub.IsValidContract(k.NameContractID)
	if err != nil {
		return err
	}
	if !valid {
		k.NameContractID = 0
	}
	if k.NameContractID == 0 && len(k.NodeDeploymentID) == 0 {
		// delete resource in case nothing is active (reflects only on read)
		k.ID = ""
	}
	return nil
}

func (k *GatewayNameDeployer) sync(ctx context.Context, sub subi.SubstrateExt ) (err error) {
	if err := k.syncContracts(ctx, sub); err != nil {
		return errors.Wrap(err, "couldn't sync contracts")
	}
	dls, err := k.deployer.GetDeployments(ctx, sub, k.NodeDeploymentID)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}
	dl := dls[k.Node]
	wl, _ := dl.Get(gridtypes.Name(k.Gw.Name))
	k.Gw = workloads.GatewayNameProxy{}
	// if the node acknowledges it, we are golden
	if wl != nil && wl.Result.State.IsOkay() {
		k.Gw, err = workloads.NewGatewayNameProxyFromZosWorkload(*wl.Workload)
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *GatewayNameDeployer) Cancel(ctx context.Context, sub subi.SubstrateExt) (err error) {
	newDeployments := make(map[uint32]gridtypes.Deployment)
	newDeploymentsData := make(map[uint32]DeploymentData)
	newDeploymentsSolutionProvider := make(map[uint32]*uint64)
	k.NodeDeploymentID, err = k.deployer.Deploy(ctx, sub, k.NodeDeploymentID, newDeployments,newDeploymentsData,newDeploymentsSolutionProvider)
	if err != nil {
		return err
	}
	if k.NameContractID != 0 {
		if err := sub.EnsureContractCanceled(k.TFPluginClient.Identity, k.NameContractID); err != nil {
			return err
		}
		k.NameContractID = 0
	}
	return nil
}