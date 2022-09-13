package storage

import (
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
)

type StorageInterface interface {
	// returns stored state
	Get() (State, error)
	// sets the state using contracts ids and user data
	Set(map[uint32]uint64, UserData) error
	ExportDeployments(map[uint32]uint64, client.NodeClientCollection, subi.ManagerInterface, string) error
}

type UserData struct {
	// map of network name to network data
	Networks map[string]NetworkData `json:"networks"`
}

// contains user secret key of a network
type NetworkData struct {
	SecretKey string `json:"secret_key"`
}

// contains contract ids and user data
type State struct {
	ContractIDs map[uint32]uint64 `json:"contract_ids"`
	UserData    UserData          `json:"user_data"`
}
