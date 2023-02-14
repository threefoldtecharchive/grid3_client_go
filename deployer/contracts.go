// Package deployer for grid deployer
package deployer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/threefoldtech/grid3-go/workloads"
)

// ContractsGetter for contracts getter from graphql
type ContractsGetter struct {
	twinID  uint32
	graphql graphQl
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
	NodeID uint64 `json:"nodeID"`
	// for name contracts
	Name string `json:"name"`
}

// NewContractsGetter return a new Getter for contracts
func NewContractsGetter(twinID uint32, graphql graphQl) ContractsGetter {
	return ContractsGetter{
		twinID:  twinID,
		graphql: graphql,
	}
}

// ListContractsByTwinID returns contracts for a twinID
func (c *ContractsGetter) ListContractsByTwinID(states []string) (Contracts, error) {
	state := fmt.Sprintf(`[%v]`, strings.Join(states, ", "))
	options := fmt.Sprintf(`(where: {twinID_eq: %v, state_in: %v}, orderBy: twinID_ASC)`, c.twinID, state)

	nameContractsCount, err := c.graphql.getItemTotalCount("nameContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	nodeContractsCount, err := c.graphql.getItemTotalCount("nodeContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	rentContractsCount, err := c.graphql.getItemTotalCount("rentContracts", options)
	if err != nil {
		return Contracts{}, err
	}

	contractsData, err := c.graphql.query(fmt.Sprintf(`query getContracts($nameContractsCount: Int!, $nodeContractsCount: Int!, $rentContractsCount: Int!){
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
		deploymentData, err := contract.parseContractDeploymentDate()
		if err != nil {
			return Contracts{}, err
		}

		if deploymentData.ProjectName == projectName {
			contracts.NodeContracts = append(contracts.NodeContracts, contract)
		}
	}

	for _, contract := range contractsList.NameContracts {
		deploymentData, err := contract.parseContractDeploymentDate()
		if err != nil {
			return Contracts{}, err
		}

		if deploymentData.ProjectName == projectName {
			contracts.NameContracts = append(contracts.NodeContracts, contract)
		}
	}

	return contracts, nil
}

func (c *Contract) parseContractDeploymentDate() (workloads.DeploymentData, error) {
	var deploymentData workloads.DeploymentData
	err := json.Unmarshal([]byte(c.DeploymentData), &deploymentData)
	if err != nil {
		return workloads.DeploymentData{}, err
	}

	return deploymentData, nil
}
