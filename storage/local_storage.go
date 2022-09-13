package storage

import (
	"context"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

type localStorage struct {
	directory string
}

func NewLocalStorage(directory string) *localStorage {
	return &localStorage{
		directory: directory,
	}
}

func (l *localStorage) Set(contractIDs map[uint32]uint64, userData UserData) error {
	state := State{
		ContractIDs: contractIDs,
		UserData:    userData,
	}
	out, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}
	err = os.MkdirAll(l.directory, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create directory: %s", l.directory)
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

func (l *localStorage) ExportDeployments(
	contractIDs map[uint32]uint64,
	ncPool client.NodeClientCollection,
	sub subi.ManagerInterface,
	directory string) error {

	s, err := sub.SubstrateExt()
	if err != nil {
		return err
	}
	defer s.Close()

	deployments := map[uint32]gridtypes.Deployment{}
	for nodeID, contractID := range contractIDs {
		nodeClient, err := ncPool.GetNodeClient(s, nodeID)
		if err != nil {
			return errors.Wrapf(err, "couldn't get node client for node: %d", nodeID)
		}
		deployment, err := nodeClient.DeploymentGet(context.Background(), contractID)
		if err != nil {
			return errors.Wrapf(err, "couldn't get deployment: %d on node: %d", contractID, nodeID)
		}
		deployments[nodeID] = deployment
	}

	out, err := json.MarshalIndent(deployments, "", "    ")
	if err != nil {
		return err
	}
	err = os.MkdirAll(directory, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create directory: %s", directory)
	}
	f, err := os.Create(directory + "deployments.json")
	if err != nil {
		return errors.Wrap(err, "couldn't create deployments file")
	}
	_, err = f.Write(out)
	return err
}
