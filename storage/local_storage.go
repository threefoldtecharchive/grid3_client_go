package storage

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
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
