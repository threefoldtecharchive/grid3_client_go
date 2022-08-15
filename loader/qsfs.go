package loader

import (
	"encoding/hex"
	"log"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func NewQSFSFromWorkload(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.QSFS, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.QSFS{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	var data *zos.QuantumSafeFS
	wd, err := wl.WorkloadData()
	if err != nil {
		return workloads.QSFS{}, err
	}
	var res zos.QuatumSafeFSResult
	if err := wl.Result.Unmarshal(&res); err != nil {
		return workloads.QSFS{}, err
	}

	log.Printf("wl.Result.unm: %s %s\n", res.MetricsEndpoint, res.Path)
	data = wd.(*zos.QuantumSafeFS)
	return workloads.QSFS{
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
		Metadata: workloads.Metadata{
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

func BackendsFromZos(bs []zos.ZdbBackend) workloads.Backends {
	z := make(workloads.Backends, 0)
	for _, e := range bs {
		z = append(z, workloads.Backend(e))
	}
	return z
}

func GroupsFromZos(gs []zos.ZdbGroup) workloads.Groups {
	z := make(workloads.Groups, 0)
	for _, e := range gs {
		z = append(z, workloads.Group{
			Backends: BackendsFromZos(e.Backends),
		})
	}
	return z
}
