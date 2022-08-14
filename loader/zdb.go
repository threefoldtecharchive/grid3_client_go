package loader

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func NewZDBFromWorkload(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.ZDB, error) {
	workload := gridtypes.Workload{}
	dataI, err := workload.WorkloadData()
	if err != nil {
		return workloads.ZDB{}, errors.Wrap(err, "failed to get workload data")
	}
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	data, ok := dataI.(*zos.ZDB)
	if !ok {
		//??
		return workloads.ZDB{}, errors.New("couldn't cast workload data")

	}
	var result zos.ZDBResult

	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return workloads.ZDB{}, errors.Wrapf(err, "failed to get zdb result")
	}
	return workloads.ZDB{
		NodeId:      nodeID,
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
