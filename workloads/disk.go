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

func (d *Disk) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloads := make([]gridtypes.Workload, 0)
	workload := gridtypes.Workload{
		Name:        gridtypes.Name(d.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: d.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
		}),
	}
	workloads = append(workloads, workload)
	for _, w := range workloads {
		err := manager.SetWorkload(NodeId, w)
		if err != nil {
			return err
		}
	}

	return nil
}
