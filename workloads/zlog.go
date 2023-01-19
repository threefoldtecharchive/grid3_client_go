// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// Zlog logger struct
type Zlog struct {
	Zmachine string
	Output   string
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

		data, ok := dataI.(*zos.ZLogs)
		if !ok {
			continue
		}

		if data.ZMachine.String() != name {
			continue
		}

		res = append(res, Zlog{
			Output:   data.Output,
			Zmachine: name,
		})
	}
	return res
}

// GenerateWorkloads generates a zmachine workload
func (zlog *Zlog) GenerateWorkloads() ([]gridtypes.Workload, error) {
	url := []byte(zlog.Output)
	urlHash := md5.Sum([]byte(url))

	return []gridtypes.Workload{
		{
			Version: 0,
			Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
			Type:    zos.ZLogsType,
			Data: gridtypes.MustMarshal(zos.ZLogs{
				ZMachine: gridtypes.Name(zlog.Zmachine),
				Output:   zlog.Output,
			}),
		},
	}, nil
}

// Stage for staging workloads
func (zlog *Zlog) GenerateNodeWorkloadsMap(nodeID uint32) (map[uint32][]gridtypes.Workload, error) {
	workloadsMap := map[uint32][]gridtypes.Workload{}

	workloads, err := zlog.GenerateWorkloads()
	if err != nil {
		return workloadsMap, err
	}

	workloadsMap[nodeID] = workloads

	return workloadsMap, nil
}
