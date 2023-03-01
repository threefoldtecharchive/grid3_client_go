// package cmd for handling commands
package cmd

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// GetVM gets deployed vm
func GetVM(name string) (workloads.VM, error) {
	path, err := config.GetConfigPath()
	if err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to get configuration file")
	}

	var cfg config.Config
	err = cfg.Load(path)
	if err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to load configuration try to login again using gridify login")
	}
	tfclient, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return workloads.VM{}, err
	}
	contracts, err := tfclient.ContractsGetter.ListContractsOfProjectName(name)
	var contractID uint64
	var nodeID uint32

	contractsSlice := append(contracts.NameContracts, contracts.NodeContracts...)
	for _, contract := range contractsSlice {
		var deploymentData workloads.DeploymentData
		err := json.Unmarshal([]byte(contract.DeploymentData), &deploymentData)
		if err != nil {
			return workloads.VM{}, errors.Wrapf(err, "failed to unmarshal deployment data %s", contract.DeploymentData)
		}
		if deploymentData.Type == "vm" && deploymentData.ProjectName == name {
			contractID, err = strconv.ParseUint(contract.ContractID, 0, 64)
			if err != nil {
				return workloads.VM{}, errors.Wrapf(err, "could not parse contract %s into uint64", contract.ContractID)
			}
			nodeID = contract.NodeID
			break
		}
	}
	nodeClient, err := tfclient.NcPool.GetNodeClient(tfclient.SubstrateConn, nodeID)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to get node client for node %d", nodeID)
	}
	dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to get deployment %d from node %d", contractID, nodeID)
	}
	var workloadVM gridtypes.Workload
	for _, workload := range dl.Workloads {
		if workload.Name == gridtypes.Name(name) {
			workloadVM = workload
			break
		}
	}
	vm, err := workloads.NewVMFromWorkload(&workloadVM, &dl)
	if err != nil {
		return workloads.VM{}, err
	}
	return vm, nil
}
