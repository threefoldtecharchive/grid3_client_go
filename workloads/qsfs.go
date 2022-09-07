package workloads

import (
	"encoding/hex"

	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type QSFS struct {
	Name                 string
	Description          string
	Cache                int
	MinimalShards        uint32
	ExpectedShards       uint32
	RedundantGroups      uint32
	RedundantNodes       uint32
	MaxZDBDataDirSize    uint32
	EncryptionAlgorithm  string
	EncryptionKey        string
	CompressionAlgorithm string
	Metadata             Metadata
	Groups               Groups

	MetricsEndpoint string
}
type Metadata struct {
	Type                string
	Prefix              string
	EncryptionAlgorithm string
	EncryptionKey       string
	Backends            Backends
}
type Group struct {
	Backends Backends
}
type Backend zos.ZdbBackend
type Groups []Group
type Backends []Backend

func (g *Group) zosGroup() zos.ZdbGroup {
	z := zos.ZdbGroup{
		Backends: make([]zos.ZdbBackend, 0),
	}
	for _, b := range g.Backends {
		z.Backends = append(z.Backends, b.zosBackend())
	}
	return z
}
func (g *Groups) zosGroups() []zos.ZdbGroup {
	z := make([]zos.ZdbGroup, 0)
	for _, e := range *g {
		z = append(z, e.zosGroup())
	}
	return z
}
func (b *Backend) zosBackend() zos.ZdbBackend {
	return zos.ZdbBackend(*b)
}
func (b *Backends) zosBackends() []zos.ZdbBackend {
	z := make([]zos.ZdbBackend, 0)
	for _, e := range *b {
		z = append(z, e.zosBackend())
	}
	return z
}

func (q *QSFS) GenerateWorkloadFromQSFS(qsfs QSFS) (gridtypes.Workload, error) {
	k, err := hex.DecodeString(qsfs.EncryptionKey)
	if err != nil {
		return gridtypes.Workload{}, err
	}
	mk, err := hex.DecodeString(qsfs.EncryptionKey)
	if err != nil {
		// return gridtypes.Workload{}, err
		return gridtypes.Workload{}, err
	}
	return gridtypes.Workload{
		Version:     0,
		Name:        gridtypes.Name(qsfs.Name),
		Type:        zos.QuantumSafeFSType,
		Description: qsfs.Description,
		Data: gridtypes.MustMarshal(zos.QuantumSafeFS{
			Cache: gridtypes.Unit(uint64(qsfs.Cache) * uint64(gridtypes.Megabyte)),
			Config: zos.QuantumSafeFSConfig{
				MinimalShards:     qsfs.MinimalShards,
				ExpectedShards:    qsfs.ExpectedShards,
				RedundantGroups:   qsfs.RedundantGroups,
				RedundantNodes:    qsfs.RedundantNodes,
				MaxZDBDataDirSize: qsfs.MaxZDBDataDirSize,
				Encryption: zos.Encryption{
					Algorithm: zos.EncryptionAlgorithm(qsfs.EncryptionAlgorithm),
					Key:       zos.EncryptionKey(k),
				},
				Meta: zos.QuantumSafeMeta{
					Type: qsfs.Metadata.Type,
					Config: zos.QuantumSafeConfig{
						Prefix: qsfs.Metadata.Prefix,
						Encryption: zos.Encryption{
							Algorithm: zos.EncryptionAlgorithm(qsfs.EncryptionAlgorithm),
							Key:       zos.EncryptionKey(mk),
						},
						Backends: qsfs.Metadata.Backends.zosBackends(),
					},
				},
				Groups: qsfs.Groups.zosGroups(),
				Compression: zos.QuantumCompression{
					Algorithm: qsfs.CompressionAlgorithm,
				},
			},
		}),
	}, nil
}

func (q *QSFS) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads := make([]gridtypes.Workload, 0)
	workload, err := q.GenerateWorkloadFromQSFS(*q)
	if err != nil {
		return err
	}
	workloads = append(workloads, workload)
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err

}
