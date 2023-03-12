// Package cmd for handling commands
package cmd

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
)

// Cancel cancels a deployed project
func Cancel(projectName string) error {
	log.Info().Msgf("canceling contracts for project %s", projectName)
	path, err := config.GetConfigPath()
	if err != nil {
		return errors.Wrap(err, "failed to get configuration file")
	}

	cfg := config.Config{}
	err = cfg.Load(path)
	if err != nil {
		return errors.Wrap(err, "failed to load configuration try to login again using tf-grid login")
	}
	tfclient, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return err
	}
	contracts, err := tfclient.ContractsGetter.ListContractsOfProjectName(projectName)
	if err != nil {
		return errors.Wrapf(err, "could not load contracts for project %s", projectName)
	}

	contractsSlice := append(contracts.NameContracts, contracts.NodeContracts...)
	for _, contract := range contractsSlice {
		contractID, err := strconv.ParseUint(contract.ContractID, 0, 64)
		if err != nil {
			return errors.Wrapf(err, "could not parse contract %s into uint64", contract.ContractID)
		}
		log.Debug().Msgf("canceling contract %d", contractID)
		err = tfclient.SubstrateConn.CancelContract(tfclient.Identity, contractID)
		if err != nil {
			return errors.Wrapf(err, "could not cancel contract %d", contractID)
		}
	}
	log.Info().Msgf("%s canceled", projectName)
	return nil
}
