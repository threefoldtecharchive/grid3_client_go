// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// Zlog logger struct
type Zlog struct {
	Output string
}

func zlogs(dl *gridtypes.Deployment, name string) []Zlog {
	var res []Zlog
	for _, wl := range dl.ByType(zos.ZLogsType) {
		if !wl.Result.State.IsOkay() {
			continue
		}
		dataI, err := wl.WorkloadData()
		if err != nil {
			continue
		}
		data := dataI.(*zos.ZLogs)
		if data.ZMachine.String() != name {
			continue
		}
		res = append(res, Zlog{
			Output: data.Output,
		})
	}
	return res
}

// GenerateWorkload generates a zmachine workload
func (zlog *Zlog) GenerateWorkload(zmachine string) (gridtypes.Workload, error) {
	url := []byte(zlog.Output)
	urlHash := md5.Sum([]byte(url))

	return gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name(zmachine),
			Output:   zlog.Output,
		}),
	}, nil
}

// Stage for staging workloads
func (zlog *Zlog) Stage(manager deployer.DeploymentManager, NodeID uint32, zmachine string) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)

	workload, err := zlog.GenerateWorkload(zmachine)
	if err != nil {
		return err
	}

	workloads = append(workloads, workload)
	workloadsMap[NodeID] = workloads

	err = manager.SetWorkloads(workloadsMap)
	return err
}
