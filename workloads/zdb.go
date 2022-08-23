package workloads

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

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

func (z *ZDB) GenerateWorkloadFromZDB(zdb ZDB) (gridtypes.Workload, error) {
	return gridtypes.Workload{
		Name:        gridtypes.Name(zdb.Name),
		Type:        zos.ZDBType,
		Description: zdb.Description,
		Version:     0,
		Data: gridtypes.MustMarshal(zos.ZDB{
			Size:     gridtypes.Unit(zdb.Size) * gridtypes.Gigabyte,
			Mode:     zos.ZDBMode(zdb.Mode),
			Password: zdb.Password,
			Public:   zdb.Public,
		}),
	}, nil
}

func (z *ZDB) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := z.GenerateWorkloadFromZDB(*z)
	workloads = append(workloads, workload)
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}
