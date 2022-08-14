package workloads

import (
	"encoding/hex"
	"log"

	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type QSFS struct {
	NodeId               uint32
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

func (q *QSFS) Stage(manager deployer.DeploymentManager) error {
	k, err := hex.DecodeString(q.EncryptionKey)
	if err != nil {
		// return gridtypes.Workload{}, err
		return err
	}
	mk, err := hex.DecodeString(q.EncryptionKey)
	if err != nil {
		// return gridtypes.Workload{}, err
		return err
	}
	workload := gridtypes.Workload{
		Version:     0,
		Name:        gridtypes.Name(q.Name),
		Type:        zos.QuantumSafeFSType,
		Description: q.Description,
		Data: gridtypes.MustMarshal(zos.QuantumSafeFS{
			Cache: gridtypes.Unit(uint64(q.Cache) * uint64(gridtypes.Megabyte)),
			Config: zos.QuantumSafeFSConfig{
				MinimalShards:     q.MinimalShards,
				ExpectedShards:    q.ExpectedShards,
				RedundantGroups:   q.RedundantGroups,
				RedundantNodes:    q.RedundantNodes,
				MaxZDBDataDirSize: q.MaxZDBDataDirSize,
				Encryption: zos.Encryption{
					Algorithm: zos.EncryptionAlgorithm(q.EncryptionAlgorithm),
					Key:       zos.EncryptionKey(k),
				},
				Meta: zos.QuantumSafeMeta{
					Type: q.Metadata.Type,
					Config: zos.QuantumSafeConfig{
						Prefix: q.Metadata.Prefix,
						Encryption: zos.Encryption{
							Algorithm: zos.EncryptionAlgorithm(q.EncryptionAlgorithm),
							Key:       zos.EncryptionKey(mk),
						},
						Backends: q.Metadata.Backends.zosBackends(),
					},
				},
				Groups: q.Groups.zosGroups(),
				Compression: zos.QuantumCompression{
					Algorithm: q.CompressionAlgorithm,
				},
			},
		}),
	}

	err = manager.SetWorkload(q.NodeId, workload)
	return err

}
