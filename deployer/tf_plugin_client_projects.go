// Package deployer for grid deployer
package deployer

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
