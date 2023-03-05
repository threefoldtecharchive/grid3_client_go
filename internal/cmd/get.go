// Package cmd for handling commands
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// GetVM returns deployed vm
func GetVM(name string) (workloads.VM, error) {

	workloadVM, dl, err := getProjectWorkload(name, "vm")
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "failed to get vm %s", name)
	}
	return workloads.NewVMFromWorkload(&workloadVM, &dl)
}

// GetGatewayName returns deployed gateway name
func GetGatewayName(name string) (workloads.GatewayNameProxy, error) {
	workloadGateway, _, err := getProjectWorkload(name, "Gateway Name")
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "failed to get gateway name %s", name)
	}
	return workloads.NewGatewayNameProxyFromZosWorkload(workloadGateway)
}

// GetGatewayFQDN returns deployed gateway fqdn
func GetGatewayFQDN(name string) (workloads.GatewayFQDNProxy, error) {
	workloadGateway, _, err := getProjectWorkload(name, "Gateway Fqdn")
	if err != nil {
		return workloads.GatewayFQDNProxy{}, errors.Wrapf(err, "failed to get gateway fqdn %s", name)
	}
	return workloads.NewGatewayFQDNProxyFromZosWorkload(workloadGateway)
}

func getProjectWorkload(name, workload string) (gridtypes.Workload, gridtypes.Deployment, error) {
	path, err := config.GetConfigPath()
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrap(err, "failed to get configuration file")
	}

	var cfg config.Config
	err = cfg.Load(path)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrap(err, "failed to load configuration try to login again using gridify login")
	}

	tfclient, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, err
	}
	contracts, err := tfclient.ContractsGetter.ListContractsOfProjectName(name)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not load contracts for project %s", name)
	}
	var contractID uint64
	var nodeID uint32

	for _, contract := range contracts.NodeContracts {
		var deploymentData workloads.DeploymentData
		err := json.Unmarshal([]byte(contract.DeploymentData), &deploymentData)
		if err != nil {
			return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "failed to unmarshal deployment data %s", contract.DeploymentData)
		}
		if deploymentData.Type == workload && deploymentData.ProjectName == name {
			contractID, err = strconv.ParseUint(contract.ContractID, 0, 64)
			if err != nil {
				return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "could not parse contract %s into uint64", contract.ContractID)
			}
			nodeID = contract.NodeID
			break
		}
	}
	if nodeID == 0 {
		return gridtypes.Workload{}, gridtypes.Deployment{}, fmt.Errorf("failed to get workload of type %s and name %s", workload, name)
	}
	nodeClient, err := tfclient.NcPool.GetNodeClient(tfclient.SubstrateConn, nodeID)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "failed to get node client for node %d", nodeID)
	}
	dl, err := nodeClient.DeploymentGet(context.Background(), contractID)
	if err != nil {
		return gridtypes.Workload{}, gridtypes.Deployment{}, errors.Wrapf(err, "failed to get deployment %d from node %d", contractID, nodeID)
	}
	var workloadRes gridtypes.Workload
	for _, workload := range dl.Workloads {
		if workload.Name == gridtypes.Name(name) {
			workloadRes = workload
			break
		}
	}
	return workloadRes, dl, nil
}
