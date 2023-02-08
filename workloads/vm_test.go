// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// VMWorkload for tests
var VMWorkload = VM{
	Name:       "test",
	Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
	CPU:        2,
	PublicIP:   true,
	Planetary:  true,
	Memory:     1024,
	RootfsSize: 20 * 1024,
	Entrypoint: "/sbin/zinit init",
	EnvVars: map[string]string{
		"SSH_KEY": "",
	},
	IP:          "10.20.2.5",
	NetworkName: "testingNetwork",
}

func TestVMWorkload(t *testing.T) {
	var workloadsFromVM []gridtypes.Workload
	var vmWorkload gridtypes.Workload

	VMWorkload.Zlogs = []Zlog{ZlogWorkload}

	pubIPWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("testip"),
		Type:    zos.PublicIPType,
		Data: gridtypes.MustMarshal(zos.PublicIP{
			V4: true,
			V6: false,
		}),
	}

	pubIPWorkload.Result.State = "ok"
	deployment := NewGridDeployment(1, []gridtypes.Workload{vmWorkload, pubIPWorkload})

	t.Run("test vm from/to map", func(t *testing.T) {
		vmFromSchema := NewVMFromSchema(VMWorkload.ToMap())
		assert.Equal(t, *vmFromSchema, VMWorkload)
	})

	t.Run("test_vm_from_workload", func(t *testing.T) {
		workloadsFromVM = VMWorkload.ZosWorkload()
		vmWorkload = workloadsFromVM[2]

		res, err := json.Marshal(zos.ZMachineResult{})
		assert.NoError(t, err)
		vmWorkload.Result.Data = res

		vmFromWorkload, err := NewVMFromWorkload(&vmWorkload, &deployment)
		assert.NoError(t, err)

		// no result yet so they are set manually
		vmFromWorkload.Planetary = true
		vmFromWorkload.PublicIP = true
		vmFromWorkload.Zlogs = []Zlog{ZlogWorkload}

		assert.Equal(t, vmFromWorkload, VMWorkload)
	})

	t.Run("test_pubIP_from_deployment", func(t *testing.T) {
		pubIP := pubIP(&deployment, "testip")
		assert.Equal(t, pubIP.HasIPv6(), false)
	})

	t.Run("test_mounts", func(t *testing.T) {
		dataI, err := vmWorkload.WorkloadData()
		assert.NoError(t, err)

		zosZmachine, ok := dataI.(*zos.ZMachine)
		assert.True(t, ok)

		mountsOfVMWorkload := mounts(zosZmachine.Mounts)
		assert.Equal(t, mountsOfVMWorkload, VMWorkload.Mounts)
	})

	t.Run("test_vm_validate", func(t *testing.T) {
		assert.NoError(t, VMWorkload.Validate())
	})

	t.Run("test_vm_failed_validate", func(t *testing.T) {
		VMWorkload.CPU = 0
		assert.ErrorIs(t, VMWorkload.Validate(), ErrInvalidInput)
		VMWorkload.CPU = 2
	})

	t.Run("test_vm_set_network_name", func(t *testing.T) {
		vmWithNetwork := VMWorkload.WithNetworkName("net")
		assert.Equal(t, vmWithNetwork.NetworkName, "net")
	})

	t.Run("test_vm_matches_another_vm", func(t *testing.T) {
		vm2 := VMWorkload.WithNetworkName("net")
		VMWorkload.LoadFromVM(vm2)
		assert.Equal(t, *vm2, VMWorkload)

		// reset network name
		vm2 = VMWorkload.WithNetworkName("testingNetwork")
		VMWorkload.LoadFromVM(vm2)
		assert.Equal(t, *vm2, VMWorkload)
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], workloadsFromVM...)

		workloadsMap2, err := VMWorkload.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
