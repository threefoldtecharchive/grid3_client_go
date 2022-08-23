package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type Disk struct {
	Name        string
	Size        int
	Description string
}

func (d *Disk) GenerateWorkloadFromDisk(disk Disk) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Name:        gridtypes.Name(disk.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: disk.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(disk.Size) * gridtypes.Gigabyte,
		}),
	}, nil
}

func (d *Disk) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := d.GenerateWorkloadFromDisk(*d)
	if err != nil {
		return err
	}
	workloads = append(workloads, workload)
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}
