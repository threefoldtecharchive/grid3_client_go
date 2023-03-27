// Package deployer for grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// CancelByProjectName cancels a deployed project
func (t *TFPluginClient) CancelByProjectName(projectName string) error {
	log.Info().Msgf("canceling contracts for project %s", projectName)
	contracts, err := t.ContractsGetter.ListContractsOfProjectName(projectName)
	if err != nil {
		return errors.Wrapf(err, "could not load contracts for project %s", projectName)
	}

	contractsSlice := append(contracts.NameContracts, contracts.NodeContracts...)
	for _, contract := range contractsSlice {
		contractID, err := strconv.ParseUint(contract.ContractID, 0, 64)
		if err != nil {
			return errors.Wrapf(err, "could not parse contract %s into uint64", contract.ContractID)
		}
		log.Debug().Uint64("canceling contract", contractID)
		err = t.SubstrateConn.CancelContract(t.Identity, contractID)
		if err != nil {
			return errors.Wrapf(err, "could not cancel contract %d", contractID)
		}
	}
	log.Info().Msgf("%s canceled", projectName)
	return nil
}

func (t *TFPluginClient) getProjectWorkload(projectName, workload string) (gridtypes.Workload, gridtypes.Deployment, uint32, error) {
	contracts, err := t.ContractsGetter.ListContractsOfProjectName(projectName)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, 0, errors.Wrapf(err, "could not load contracts for project %s", projectName)
	}
	var contractID uint64
	var nodeID uint32

	for _, contract := range contracts.NodeContracts {
		var deploymentData workloads.DeploymentData
		err := json.Unmarshal([]byte(contract.DeploymentData), &deploymentData)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, 0, errors.Wrapf(err, "failed to unmarshal deployment data %s", contract.DeploymentData)
		}
		if deploymentData.Type != workload || deploymentData.ProjectName != projectName {
			continue
		}
		contractID, err = strconv.ParseUint(contract.ContractID, 0, 64)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, 0, errors.Wrapf(err, "could not parse contract %s into uint64", contract.ContractID)
		}
		nodeID = contract.NodeID
		nodeClient, err := t.NcPool.GetNodeClient(t.SubstrateConn, nodeID)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, 0, errors.Wrapf(err, "failed to get node client for node %d", nodeID)
		}
		dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, 0, errors.Wrapf(err, "failed to get deployment %d from node %d", contractID, nodeID)
		}
		for _, workload := range dl.Workloads {
			if workload.Name == gridtypes.Name(projectName) {
				return workload, dl, nodeID, nil
			}
		}

	}
	return gridtypes.Workload{}, gridtypes.Deployment{}, 0, fmt.Errorf("failed to get workload of type %s and name %s", workload, projectName)
}
