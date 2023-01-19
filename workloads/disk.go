// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// Disk struct
type Disk struct {
	Name        string
	Size        int
	Description string
}

// NewDiskFromSchema converts a disk data map to a struct
func NewDiskFromSchema(disk map[string]interface{}) Disk {
	return Disk{
		Name:        disk["name"].(string),
		Size:        disk["size"].(int),
		Description: disk["description"].(string),
	}
}

// NewDiskFromWorkload generates a new disk from a workload
func NewDiskFromWorkload(wl *gridtypes.Workload) (Disk, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return Disk{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.ZMount)
	if !ok {
		return Disk{}, errors.New("couldn't cast workload data")
	}

	return Disk{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Size:        int(data.Size / gridtypes.Gigabyte),
	}, nil
}

// Dictify converts a disk data to a map
func (d *Disk) Dictify() map[string]interface{} {
	res := make(map[string]interface{})
	res["name"] = d.Name
	res["description"] = d.Description
	res["size"] = d.Size
	return res
}

// GenerateDiskWorkload generates a disk workload
func (d *Disk) GenerateDiskWorkload() gridtypes.Workload {
	workload := gridtypes.Workload{
		Name:        gridtypes.Name(d.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: d.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
		}),
	}

	return workload
}

// GetName returns disk name
func (d *Disk) GetName() string {
	return d.Name
}

// GenerateWorkloadFromDisk generates a workload from a disk
func (d *Disk) GenerateWorkloadFromDisk() (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Name:        gridtypes.Name(d.Name),
		Version:     0,
		Type:        zos.ZMountType,
		Description: d.Description,
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
		}),
	}, nil
}

// Stage for staging workloads
func (d *Disk) Stage(manager deployer.DeploymentManager, nodeID uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := d.GenerateWorkloadFromDisk()
	if err != nil {
		return err
	}
	workloads = append(workloads, workload)
	workloadsMap[nodeID] = workloads

	err = manager.SetWorkloads(workloadsMap)
	return err
}
