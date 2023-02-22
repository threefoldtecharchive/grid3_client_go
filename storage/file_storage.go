package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

type fileStorage struct {
	directory string
}

func NewFileStorage(directory string) *fileStorage {
	if directory[len(directory)-1:] != "/" {
		directory += "/"
	}
	return &fileStorage{
		directory: directory,
	}
}

func (l *fileStorage) Set(contractIDs map[uint32]uint64, userData UserData) error {
	_, err := os.Stat(l.directory)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory: %s does not exist", l.directory)
		} else {
			return err
		}
	}
	state := State{
		ContractIDs: contractIDs,
		UserData:    userData,
	}
	out, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(l.directory + "state.json"))
	if err != nil {
		return errors.Wrap(err, "couldn't create state file")
	}
	_, err = f.Write(out)
	return err
}

func (l *fileStorage) Get() (State, error) {
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

func (l *fileStorage) ExportDeployments(
	contractIDs map[uint32]uint64,
	ncPool client.NodeClientCollection,
	sub subi.ManagerInterface,
	directory string) error {

	if directory[len(directory)-1:] != "/" {
		directory += "/"
	}
	_, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory: %s does not exist", directory)
		} else {
			return err
		}
	}

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
	f, err := os.Create(filepath.Join(directory, "deployments.json"))
	if err != nil {
		return errors.Wrap(err, "couldn't create deployments file")
	}
	_, err = f.Write(out)
	return err
}
