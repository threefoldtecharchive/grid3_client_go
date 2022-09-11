package deployer

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

type StorageInterface interface {
	Get() (State, error)
	Set(DeploymentManager, UserData) error
}

type localStorage struct {
	directory string
}

func NewLocalStorage(directory string) *localStorage {
	return &localStorage{
		directory: directory,
	}
}

type Deployment struct {
	Node uint32 `json:"node_id"`

	Version uint32 `json:"version"`

	TwinID uint32 `json:"twin_id"`

	ContractID uint64 `json:"contract_id"`

	Metadata string `json:"metadata"`

	Description string `json:"description"`

	Workloads []gridtypes.Workload `json:"workloads"`
}
type UserData struct {
	Networks map[string]NetworkData `json:"networks"`
}
type NetworkData struct {
	SecretKey string `json:"secret_key"`
}
type State struct {
	Deployments map[uint32]Deployment `json:"deployments"`
	UserData    UserData              `json:"user_data"`
}

func (l *localStorage) Set(d DeploymentManager, userData UserData) error {
	contracts := d.GetContractIDs()
	state := State{Deployments: map[uint32]Deployment{}, UserData: userData}
	for nodeID := range contracts {
		deployment, err := d.GetDeployment(nodeID)
		if err != nil {
			return errors.Wrapf(err, "couldn't get deployment %d on node %d", contracts[nodeID], nodeID)
		}
		d := Deployment{
			Node:        nodeID,
			Version:     deployment.Version,
			TwinID:      deployment.TwinID,
			ContractID:  deployment.ContractID,
			Metadata:    deployment.Metadata,
			Description: deployment.Description,
			Workloads:   deployment.Workloads,
		}
		state.Deployments[nodeID] = d

	}
	out, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}
	f, err := os.Create(l.directory + "state.json")
	if err != nil {
		return errors.Wrap(err, "couldn't create state file")
	}
	_, err = f.Write(out)
	return err
}

func (l *localStorage) Get() (State, error) {
	state := State{}
	data, err := os.ReadFile(l.directory + "state.json")
	if err != nil {
		return State{}, errors.Wrapf(err, "couldn't read state file in directory: %s", l.directory)
	}
	err = json.Unmarshal(data, &state)
	if err != nil {
		return State{}, err
	}
	return state, nil
}
