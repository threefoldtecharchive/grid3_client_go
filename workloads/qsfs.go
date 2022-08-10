package workloads

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type QSFS struct {
	NodeID               uint32
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

func BackendsFromZos(bs []zos.ZdbBackend) Backends {
	z := make(Backends, 0)
	for _, e := range bs {
		z = append(z, Backend(e))
	}
	return z
}

func GroupsFromZos(gs []zos.ZdbGroup) Groups {
	z := make(Groups, 0)
	for _, e := range gs {
		z = append(z, Group{
			Backends: BackendsFromZos(e.Backends),
		})
	}
	return z
}

func NewQSFSFromSchema(qsfs map[string]interface{}) QSFS {
	metadataIf := qsfs["metadata"].([]interface{})
	metadataMap := metadataIf[0].(map[string]interface{})

	metadata := Metadata{
		Type:                metadataMap["type"].(string),
		Prefix:              metadataMap["prefix"].(string),
		EncryptionAlgorithm: metadataMap["encryption_algorithm"].(string),
		EncryptionKey:       metadataMap["encryption_key"].(string),
		Backends:            getBackends(metadataMap["backends"].([]interface{})),
	}
	groupsIf := qsfs["groups"].([]interface{})
	groups := make([]Group, 0, len(groupsIf))
	for _, gr := range groupsIf {
		groupMap := gr.(map[string]interface{})
		groups = append(groups, Group{
			Backends: getBackends(groupMap["backends"].([]interface{})),
		})
	}
	return QSFS{
		Name:                 qsfs["name"].(string),
		Description:          qsfs["description"].(string),
		Cache:                qsfs["cache"].(int),
		MinimalShards:        uint32(qsfs["minimal_shards"].(int)),
		ExpectedShards:       uint32(qsfs["expected_shards"].(int)),
		RedundantGroups:      uint32(qsfs["redundant_groups"].(int)),
		RedundantNodes:       uint32(qsfs["redundant_nodes"].(int)),
		MaxZDBDataDirSize:    uint32(qsfs["max_zdb_data_dir_size"].(int)),
		EncryptionAlgorithm:  qsfs["encryption_algorithm"].(string),
		EncryptionKey:        qsfs["encryption_key"].(string),
		CompressionAlgorithm: qsfs["compression_algorithm"].(string),
		Metadata:             metadata,
		Groups:               groups,
	}
}

func NewQSFSFromWorkload(wl *gridtypes.Workload) (QSFS, error) {

	wd, err := wl.WorkloadData()
	if err != nil {
		return QSFS{}, err
	}
	var res zos.QuatumSafeFSResult
	if err := wl.Result.Unmarshal(&res); err != nil {
		return QSFS{}, err
	}
	data, ok := wd.(*zos.QuantumSafeFS)
	if !ok {
		return QSFS{}, errors.New("wrong workload type")
	}
	return QSFS{
		Name:                 string(wl.Name),
		Description:          string(wl.Description),
		Cache:                int(data.Cache) / int(gridtypes.Megabyte),
		MinimalShards:        data.Config.MinimalShards,
		ExpectedShards:       data.Config.ExpectedShards,
		RedundantGroups:      data.Config.RedundantGroups,
		RedundantNodes:       data.Config.RedundantNodes,
		MaxZDBDataDirSize:    data.Config.MaxZDBDataDirSize,
		EncryptionAlgorithm:  string(data.Config.Encryption.Algorithm),
		EncryptionKey:        hex.EncodeToString(data.Config.Encryption.Key),
		CompressionAlgorithm: data.Config.Compression.Algorithm,
		Metadata: Metadata{
			Type:                data.Config.Meta.Type,
			Prefix:              data.Config.Meta.Config.Prefix,
			EncryptionAlgorithm: string(data.Config.Meta.Config.Encryption.Algorithm),
			EncryptionKey:       hex.EncodeToString(data.Config.Meta.Config.Encryption.Key),
			Backends:            BackendsFromZos(data.Config.Meta.Config.Backends),
		},
		Groups:          GroupsFromZos(data.Config.Groups),
		MetricsEndpoint: res.MetricsEndpoint,
	}, nil
}

func getBackends(backendsIf []interface{}) []Backend {
	backends := make([]Backend, 0, len(backendsIf))
	for _, b := range backendsIf {
		backendMap := b.(map[string]interface{})
		backends = append(backends, Backend{
			Address:   backendMap["address"].(string),
			Password:  backendMap["password"].(string),
			Namespace: backendMap["namespace"].(string),
		})
	}
	return backends
}

func (q *QSFS) Convert(d deployer.DeploymentManager) error {
	k, err := hex.DecodeString(q.EncryptionKey)
	if err != nil {
		return err
	}
	mk, err := hex.DecodeString(q.EncryptionKey)
	if err != nil {
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
	err = d.SetWorkload(q.NodeID, workload)
	return err
}
