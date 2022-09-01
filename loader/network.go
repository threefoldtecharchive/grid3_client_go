package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type LoadedNetwork struct {
	Name          string
	Description   string
	NodeNetwork   map[uint32]zos.Network
	NodeContracts map[uint32]uint64
}

func LoadNetworkFromGrid(manager deployer.DeploymentManager, name string) (LoadedNetwork, error) {
	ret := LoadedNetwork{
		NodeNetwork:   make(map[uint32]zos.Network),
		NodeContracts: make(map[uint32]uint64),
	}
	for nodeID, contractID := range manager.GetContractIDs() {
		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return LoadedNetwork{}, errors.Wrapf(err, "failed to get deployment with id %d", nodeID)
		}
		for _, wl := range dl.Workloads {
			if wl.Type == zos.NetworkType && wl.Name == gridtypes.Name(name) {
				ret.Name = wl.Name.String()
				ret.Description = wl.Description
				dataI, err := wl.WorkloadData()
				if err != nil {
					return LoadedNetwork{}, errors.Wrap(err, "failed to get workload data")
				}
				data, ok := dataI.(*zos.Network)
				if !ok {
					return LoadedNetwork{}, errors.New("couldn't cast workload data")
				}
				ret.NodeNetwork[nodeID] = *data
				ret.NodeContracts[nodeID] = contractID
				break
			}
		}
	}
	return ret, nil
}
