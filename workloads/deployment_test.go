// Package workloads includes workloads types (vm, zdb, QSFS, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestNewDeployment(t *testing.T) {
	var zosDeployment gridtypes.Deployment
	deployment := NewDeployment(
		"test", 1, "", nil, Network.Name,
		[]Disk{DiskWorkload},
		[]ZDB{ZDBWorkload},
		[]VM{VMWorkload},
		[]QSFS{QSFSWorkload},
	)

	t.Run("test deployment validate", func(t *testing.T) {
		assert.NoError(t, deployment.Validate())
	})

	t.Run("test zos deployment", func(t *testing.T) {
		var err error
		zosDeployment, err = deployment.ZosDeployment(1)
		assert.NoError(t, err)

		workloads := []gridtypes.Workload{DiskWorkload.ZosWorkload(), ZDBWorkload.ZosWorkload()}
		workloads = append(workloads, VMWorkload.ZosWorkload()...)
		QSFS, err := QSFSWorkload.ZosWorkload()
		assert.NoError(t, err)
		workloads = append(workloads, QSFS)

		newZosDeployment := NewGridDeployment(1, workloads)
		assert.Equal(t, newZosDeployment, zosDeployment)
	})

	t.Run("test deployment used ips", func(t *testing.T) {
		for i := range zosDeployment.Workloads {
			zosDeployment.Workloads[i].Result.State = "ok"
		}

		res, err := json.Marshal(zos.ZMachineResult{})
		assert.NoError(t, err)
		zosDeployment.Workloads[3].Result.Data = res

		usedIPs, err := GetUsedIPs(zosDeployment)
		assert.NoError(t, err)
		assert.Equal(t, usedIPs, []byte{5})
	})

	t.Run("test deployment match", func(t *testing.T) {
		dlCp := deployment
		deployment.Match([]Disk{}, []QSFS{}, []ZDB{}, []VM{})
		assert.Equal(t, deployment, dlCp)
	})

	t.Run("test deployment nullify", func(t *testing.T) {
		deployment.Nullify()
		assert.Equal(t, deployment.Vms, []VM(nil))
		assert.Equal(t, deployment.Disks, []Disk(nil))
		assert.Equal(t, deployment.QSFS, []QSFS(nil))
		assert.Equal(t, deployment.Zdbs, []ZDB(nil))
		assert.Equal(t, deployment.ContractID, uint64(0))
	})
}
