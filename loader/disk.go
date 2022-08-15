package loader

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func NewDiskFromWorkload(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.Disk, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.Disk{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}
	dataI, err := wl.WorkloadData()
	if err != nil {
		return workloads.Disk{}, errors.Wrap(err, "failed to get workload data")
	}
	// TODO: check ok?
	data := dataI.(*zos.ZMount)
	return workloads.Disk{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Size:        int(data.Size / gridtypes.Gigabyte),
	}, nil
}
