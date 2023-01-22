// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"github.com/pkg/errors"
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

// ToMap converts a disk data to a map
func (d *Disk) ToMap() map[string]interface{} {
	res := make(map[string]interface{})
	res["name"] = d.Name
	res["description"] = d.Description
	res["size"] = d.Size
	return res
}

// GetName returns disk name
func (d *Disk) GetName() string {
	return d.Name
}

// GenerateWorkloads generates a workload from a disk
func (d *Disk) GenerateWorkloads() ([]gridtypes.Workload, error) {
	return []gridtypes.Workload{
		{
			Name:        gridtypes.Name(d.Name),
			Version:     0,
			Type:        zos.ZMountType,
			Description: d.Description,
			Data: gridtypes.MustMarshal(zos.ZMount{
				Size: gridtypes.Unit(d.Size) * gridtypes.Gigabyte,
			}),
		},
	}, nil
}

// BindWorkloadsToNode for staging workloads with node ID
func (d *Disk) BindWorkloadsToNode(nodeID uint32) (map[uint32][]gridtypes.Workload, error) {
	workloadsMap := map[uint32][]gridtypes.Workload{}

	workloads, err := d.GenerateWorkloads()
	if err != nil {
		return workloadsMap, err
	}

	workloadsMap[nodeID] = workloads
	return workloadsMap, nil
}
