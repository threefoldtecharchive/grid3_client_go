package workloads

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type ZDB struct {
	NodeID      uint32
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

func GetZDBData(zdb map[string]interface{}) ZDB {
	ipsIf := zdb["ips"].([]interface{})
	ips := make([]string, len(ipsIf))
	for idx, ip := range ipsIf {
		ips[idx] = ip.(string)
	}

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

func NewZDBFromWorkload(wl *gridtypes.Workload) (ZDB, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return ZDB{}, errors.Wrap(err, "failed to get workload data")
	}
	data, ok := dataI.(*zos.ZDB)
	if !ok {
		return ZDB{}, errors.New("wrong workload type")
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

func (z *ZDB) Convert(d deployer.DeploymentManager) error {
	workload := gridtypes.Workload{
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
	err := d.SetWorkload(z.NodeID, workload)
	return err
}
