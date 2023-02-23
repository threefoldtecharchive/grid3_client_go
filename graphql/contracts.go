// Package graphql for grid graphql support
package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// ContractsGetter for contracts getter from graphql
type ContractsGetter struct {
	twinID        uint32
	graphql       GraphQl
	substrateConn subi.SubstrateExt
	ncPool        client.NodeClientGetter
}

// Contracts from graphql
type Contracts struct {
	NameContracts []Contract `json:"nameContracts"`
	NodeContracts []Contract `json:"nodeContracts"`
	RentContracts []Contract `json:"rentContracts"`
}

// Contract from graphql
type Contract struct {
	ContractID     string `json:"contractID"`
	State          string `json:"state"`
	DeploymentData string `json:"deploymentData"`

	// for node and rent contracts
	NodeID uint32 `json:"nodeID"`
	// for name contracts
	Name string `json:"name"`
}

// NewContractsGetter return a new Getter for contracts
func NewContractsGetter(twinID uint32, graphql GraphQl, substrateConn subi.SubstrateExt, ncPool client.NodeClientGetter) ContractsGetter {
	return ContractsGetter{
		twinID:        twinID,
		graphql:       graphql,
		substrateConn: substrateConn,
		ncPool:        ncPool,
	}
}

// ListContractsByTwinID returns contracts for a twinID
func (c *ContractsGetter) ListContractsByTwinID(states []string) (Contracts, error) {
	state := fmt.Sprintf(`[%v]`, strings.Join(states, ", "))
	options := fmt.Sprintf(`(where: {twinID_eq: %v, state_in: %v}, orderBy: twinID_ASC)`, c.twinID, state)

	nameContractsCount, err := c.graphql.GetItemTotalCount("nameContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	nodeContractsCount, err := c.graphql.GetItemTotalCount("nodeContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	rentContractsCount, err := c.graphql.GetItemTotalCount("rentContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	contractsData, err := c.graphql.Query(fmt.Sprintf(`query getContracts($nameContractsCount: Int!, $nodeContractsCount: Int!, $rentContractsCount: Int!){
            nameContracts(where: {twinID_eq: %v, state_in: %v}, limit: $nameContractsCount) {
              contractID
              state
              name
            }
            nodeContracts(where: {twinID_eq: %v, state_in: %v}, limit: $nodeContractsCount) {
              contractID
              deploymentData
              state
              nodeID
            }
            rentContracts(where: {twinID_eq: %v, state_in: %v}, limit: $rentContractsCount) {
              contractID
              state
              nodeID
            }
          }`, c.twinID, state, c.twinID, state, c.twinID, state),
		map[string]interface{}{
			"nodeContractsCount": nodeContractsCount,
			"nameContractsCount": nameContractsCount,
			"rentContractsCount": rentContractsCount,
		})

	if err != nil {
		return Contracts{}, err
	}

	contractsJSONData, err := json.Marshal(contractsData)
	if err != nil {
		return Contracts{}, err
	}

	var listContracts Contracts
	err = json.Unmarshal(contractsJSONData, &listContracts)
	if err != nil {
		return Contracts{}, err
	}

	return listContracts, nil
}

// ListContractsOfProjectName returns contracts for a project name
func (c *ContractsGetter) ListContractsOfProjectName(projectName string) (Contracts, error) {
	contracts := Contracts{
		NodeContracts: make([]Contract, 0),
		NameContracts: make([]Contract, 0),
	}
	contractsList, err := c.ListContractsByTwinID([]string{"Created, GracePeriod"})
	if err != nil {
		return Contracts{}, err
	}

	for _, contract := range contractsList.NodeContracts {
		deploymentData, err := workloads.ParseDeploymentDate(contract.DeploymentData)
		if err != nil {
			return Contracts{}, err
		}

		if deploymentData.ProjectName == projectName {
			contracts.NodeContracts = append(contracts.NodeContracts, contract)
		}
	}

	nameGatewaysWorkloads, err := c.filterNameGatewaysWithinNodeContracts(contracts.NodeContracts)
	if err != nil {
		return Contracts{}, err
	}

	contracts.NameContracts, err = c.filterNameContracts(contractsList.NameContracts, nameGatewaysWorkloads)
	if err != nil {
		return Contracts{}, err
	}

	return contracts, nil
}

// filterNameContracts returns the name contracts of the given name gateways
func (c *ContractsGetter) filterNameContracts(nameContracts []Contract, nameGatewayWorkloads []gridtypes.Workload) ([]Contract, error) {
	filteredNameContracts := make([]Contract, 0)
	for _, contract := range nameContracts {
		for _, w := range nameGatewayWorkloads {
			if w.Name.String() == contract.Name {
				filteredNameContracts = append(filteredNameContracts, contract)
			}
		}
	}

	return filteredNameContracts, nil
}

func (c *ContractsGetter) filterNameGatewaysWithinNodeContracts(nodeContracts []Contract) ([]gridtypes.Workload, error) {
	nameGatewayWorkloads := make([]gridtypes.Workload, 0)
	for _, contract := range nodeContracts {
		nodeClient, err := c.ncPool.GetNodeClient(c.substrateConn, contract.NodeID)
		if err != nil {
			return []gridtypes.Workload{}, errors.Wrapf(err, "could not get node client: %d", contract.NodeID)
		}

		contractID, err := strconv.Atoi(contract.ContractID)
		if err != nil {
			return []gridtypes.Workload{}, errors.Wrapf(err, "could not parse contract id: %s", contract.ContractID)
		}

		dl, err := nodeClient.DeploymentGet(context.Background(), uint64(contractID))
		if err != nil {
			return []gridtypes.Workload{}, errors.Wrapf(err, "could not get deployment %d from node %d", contractID, contract.NodeID)
		}

		for _, workload := range dl.Workloads {
			if workload.Type == zos.GatewayNameProxyType {
				nameGatewayWorkloads = append(nameGatewayWorkloads, workload)
			}
		}
	}

	return nameGatewayWorkloads, nil
}
