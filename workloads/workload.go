package workloads

import "github.com/threefoldtech/zos/pkg/gridtypes"

type Workload interface {
	Convert() []gridtypes.Workload
}
