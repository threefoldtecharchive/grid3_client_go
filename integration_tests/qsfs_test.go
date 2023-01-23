// Package integration for integration tests
package integration

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/manager"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const (
	DataZDBNum = 4
	MetaZDBNum = 4
)

func TestQSFSDeployment(t *testing.T) {
	dataZDBs := []workloads.ZDB{}
	metaZDBs := []workloads.ZDB{}
	for i := 1; i <= DataZDBNum; i++ {
		zdb := workloads.ZDB{
			Name:        "qsfsDataZdb" + strconv.Itoa(i),
			Password:    "password",
			Public:      true,
			Size:        1,
			Description: "zdb for testing",
			Mode:        zos.ZDBModeSeq,
		}
		dataZDBs = append(dataZDBs, zdb)
	}
	for i := 1; i <= MetaZDBNum; i++ {
		zdb := workloads.ZDB{
			Name:        "qsfsMetaZdb" + strconv.Itoa(i),
			Password:    "password",
			Public:      true,
			Size:        1,
			Description: "zdb for testing",
			Mode:        zos.ZDBModeUser,
		}
		metaZDBs = append(metaZDBs, zdb)
	}

	dlManager, _ := setup()
	var err error
	for i := 0; i < DataZDBNum; i++ {
		err = dlManager.Stage(&dataZDBs[i], 14)
		assert.NoError(t, err)
	}
	for i := 0; i < MetaZDBNum; i++ {
		err = dlManager.Stage(&metaZDBs[i], 14)
		assert.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	err = dlManager.Commit(ctx)
	assert.NoError(t, err)

	err = dlManager.CancelAll()
	assert.NoError(t, err)

	resDataZDBs := []workloads.ZDB{}
	resMetaZDBs := []workloads.ZDB{}
	for i := 1; i <= DataZDBNum; i++ {
		res, err := manager.LoadZdbFromGrid(dlManager, 14, "qsfsDataZdb"+strconv.Itoa(i))
		assert.NotEmpty(t, res)
		assert.NoError(t, err)
		resDataZDBs = append(resDataZDBs, res)
	}
	for i := 1; i <= MetaZDBNum; i++ {
		res, err := manager.LoadZdbFromGrid(dlManager, 14, "qsfsMetaZdb"+strconv.Itoa(i))
		assert.NotEmpty(t, res)
		assert.NoError(t, err)
		resMetaZDBs = append(resMetaZDBs, res)
	}

	dataBackends := []workloads.Backend{}
	for i := 0; i < DataZDBNum; i++ {
		dataBackends = append(dataBackends, workloads.Backend{
			Address:   "[" + resDataZDBs[i].IPs[1] + "]" + ":" + fmt.Sprint(resDataZDBs[i].Port),
			Namespace: resDataZDBs[i].Namespace,
			Password:  resDataZDBs[i].Password})
	}
	metaBackends := []workloads.Backend{}
	for i := 0; i < MetaZDBNum; i++ {
		metaBackends = append(metaBackends, workloads.Backend{
			Address:   "[" + resMetaZDBs[i].IPs[1] + "]" + ":" + fmt.Sprint(resMetaZDBs[i].Port),
			Namespace: resMetaZDBs[i].Namespace,
			Password:  resMetaZDBs[i].Password})
	}

	qsfs := workloads.QSFS{
		Name:                 "qsftTest",
		Description:          "qsfs for testing",
		Cache:                1024,
		MinimalShards:        2,
		ExpectedShards:       4,
		RedundantGroups:      0,
		RedundantNodes:       0,
		MaxZDBDataDirSize:    512,
		EncryptionAlgorithm:  "AES",
		EncryptionKey:        "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
		CompressionAlgorithm: "snappy",
		Groups:               workloads.Groups{{Backends: dataBackends}},
		Metadata: workloads.Metadata{
			Type:                "zdb",
			Prefix:              "test",
			EncryptionAlgorithm: "AES",
			EncryptionKey:       "4d778ba3216e4da4231540c92a55f06157cabba802f9b68fb0f78375d2e825af",
			Backends:            metaBackends,
		},
	}
	err = dlManager.Stage(&qsfs, 14)
	assert.NoError(t, err)
	err = dlManager.Commit(ctx)
	assert.NoError(t, err)

	resQSFS, err := manager.LoadQsfsFromGrid(dlManager, 14, "qsftTest")
	assert.NoError(t, err)
	assert.NotEmpty(t, resQSFS.MetricsEndpoint)
	resQSFS.MetricsEndpoint = ""
	assert.Equal(t, qsfs, resQSFS)

}
