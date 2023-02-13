// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// ZDB workload struct
type ZDB struct {
	Name        string
	Password    string
	Public      bool
	Size        int
	Description string
	Mode        string
	IPs         []string
	Port        uint32
	Namespace   string
}

// NewZDBFromMap converts a map including zdb data to a zdb struct
func NewZDBFromMap(zdb map[string]interface{}) ZDB {
	ips := zdb["ips"].([]string)

	return ZDB{
		Name:        zdb["name"].(string),
		Size:        zdb["size"].(int),
		Description: zdb["description"].(string),
		Password:    zdb["password"].(string),
		Public:      zdb["public"].(bool),
		Mode:        zdb["mode"].(string),
		IPs:         ips,
		Port:        uint32(zdb["port"].(int)),
		Namespace:   zdb["namespace"].(string),
	}
}

// NewZDBFromWorkload generates a new zdb from a workload
func NewZDBFromWorkload(wl *gridtypes.Workload) (ZDB, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return ZDB{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.ZDB)
	if !ok {
		return ZDB{}, fmt.Errorf("could not create zdb workload from data %v", dataI)
	}

	var result zos.ZDBResult

	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return ZDB{}, errors.Wrap(err, "failed to get zdb result")
	}

	return ZDB{
		Name:        wl.Name.String(),
		Description: wl.Description,
		Password:    data.Password,
		Public:      data.Public,
		Size:        int(data.Size / gridtypes.Gigabyte),
		Mode:        data.Mode.String(),
		IPs:         result.IPs,
		Port:        uint32(result.Port),
		Namespace:   result.Namespace,
	}, nil
}

// ToMap converts a zdb to a map(dict) object
func (z *ZDB) ToMap() map[string]interface{} {
	res := make(map[string]interface{})
	res["name"] = z.Name
	res["description"] = z.Description
	res["size"] = z.Size
	res["mode"] = z.Mode
	res["ips"] = z.IPs
	res["namespace"] = z.Namespace
	res["port"] = int(z.Port)
	res["password"] = z.Password
	res["public"] = z.Public
	return res
}

// ZosWorkload generates a workload from a zdb
func (z *ZDB) ZosWorkload() gridtypes.Workload {
	return gridtypes.Workload{
		Name:        gridtypes.Name(z.Name),
		Type:        zos.ZDBType,
		Description: z.Description,
		Version:     0,
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     gridtypes.Unit(z.Size) * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode(z.Mode),
			Password: z.Password,
			Public:   z.Public,
		}),
	}
}
