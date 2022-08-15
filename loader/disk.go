package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func LoadDiskFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.Disk, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}
	dataI, err := wl.WorkloadData()
	if err != nil {
		return workloads.Disk{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.ZMount)
	if !ok {
		return workloads.Disk{}, errors.New("couldn't cast workload data")
	}
	return workloads.Disk{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Size:        int(data.Size / gridtypes.Gigabyte),
	}, nil
}
