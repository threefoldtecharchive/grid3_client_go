package workloads

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type Disk struct {
	NodeID      uint32
	Name        string
	Size        int
	Description string
}

func GetDiskData(disk map[string]interface{}) Disk {
	return Disk{
		Name:        disk["name"].(string),
		Size:        disk["size"].(int),
		Description: disk["description"].(string),
	}
}
func NewDiskFromWorkload(wl *gridtypes.Workload) (Disk, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return Disk{}, errors.Wrap(err, "failed to get workload data")
	}
	data, ok := dataI.(*zos.ZMount)
	if !ok {
		return Disk{}, errors.New("wrong work load type")
	}
	return Disk{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Size:        int(data.Size / gridtypes.Gigabyte),
	}, nil
}

func (d *Disk) Convert(dm deployer.DeploymentManager) error {
	workload := gridtypes.Workload{
		Name:        gridtypes.Name(d.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: d.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
		}),
	}
	err := dm.SetWorkload(d.NodeID, workload)
	return err
}
