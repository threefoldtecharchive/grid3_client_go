package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type Disk struct {
	NodeId      uint32
	Name        string
	Size        int
	Description string
}

func (d *Disk) Convert(manager deployer.DeploymentManager) error {
	workload := gridtypes.Workload{
		Name:        gridtypes.Name(d.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: d.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
		}),
	}
	err := manager.SetWorkload(d.NodeId, workload)
	return err
}
