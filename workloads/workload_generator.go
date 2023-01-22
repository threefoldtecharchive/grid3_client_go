// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

// WorkloadsToNodeBinder generates a grid workload
type WorkloadsToNodeBinder interface {
	BindWorkloadsToNode(nodeID uint32) (map[uint32][]gridtypes.Workload, error)
}
