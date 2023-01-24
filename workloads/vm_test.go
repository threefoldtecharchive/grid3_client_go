// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestVMWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var vmMap map[string]interface{}

	var workloadsFromVM []gridtypes.Workload

	vm := VM{
		Name:          "test",
		Flist:         "flist test",
		FlistChecksum: "",
		PublicIP:      true,
		PublicIP6:     false,
		Planetary:     true,
		Corex:         false,
		IP:            "1.1.1.1",
		Description:   "test des",
		CPU:           2,
		Memory:        2048,
		RootfsSize:    4096,
		Entrypoint:    "entrypoint",
		Mounts: []Mount{
			{DiskName: "disk", MountPoint: "mount"},
		},
		Zlogs: []Zlog{
			{Output: "output", Zmachine: "test"},
		},
		EnvVars:     map[string]string{"var1": "val1"},
		NetworkName: "test network",
	}

	pubIPWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("testip"),
		Type:    zos.PublicIPType,
		Data: gridtypes.MustMarshal(zos.PublicIP{
			V4: true,
			V6: false,
		}),
	}

	vmRes, err := json.Marshal(zos.ZMachineResult{
		ID:    "",
		IP:    "",
		YggIP: "",
	})
	assert.NoError(t, err)

	vmWorkload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test"),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: "flist test",
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name("test network"),
						IP:      net.ParseIP("1.1.1.1"),
					},
				},
				PublicIP:  gridtypes.Name("testip"),
				Planetary: true,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(2),
				Memory: 2048 * gridtypes.Megabyte,
			},
			Size:       4096 * gridtypes.Megabyte,
			Entrypoint: "entrypoint",
			Corex:      false,
			Mounts: []zos.MachineMount{
				{Name: gridtypes.Name("disk"), Mountpoint: "mount"},
			},
			Env: map[string]string{"var1": "val1"},
		}),
		Description: "test des",
		Result: gridtypes.Result{
			Created: 5000,
			State:   gridtypes.StateOk,
			Data:    vmRes,
		},
	}

	pubIPWorkload.Result.State = "ok"
	deployment := gridtypes.Deployment{
		Version:   0,
		TwinID:    1,
		Workloads: []gridtypes.Workload{vmWorkload, pubIPWorkload},
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: 1,
					Weight: 1,
				},
			},
		},
	}

	t.Run("test_vm_dictify", func(t *testing.T) {
		vmMap = vm.ToMap()
	})

	t.Run("test_vm_from_schema", func(t *testing.T) {
		vmFromSchema := NewVMFromSchema(vmMap)
		assert.Equal(t, *vmFromSchema, vm)
	})

	t.Run("test_vm_from_workload", func(t *testing.T) {
		vmFromWorkload, err := NewVMFromWorkloads(&vmWorkload, &deployment)
		assert.NoError(t, err)

		// no result yet so they are set manually
		vmFromWorkload.Planetary = true
		vmFromWorkload.PublicIP = true
		vmFromWorkload.Zlogs = []Zlog{{
			Zmachine: "test",
			Output:   "output",
		}}

		assert.Equal(t, vmFromWorkload, vm)
	})

	t.Run("test_pubIP_from_deployment", func(t *testing.T) {
		pubIP := pubIP(&deployment, "testip")
		assert.Equal(t, pubIP.HasIPv6(), false)
	})

	t.Run("test_mounts", func(t *testing.T) {
		dataI, err := vmWorkload.WorkloadData()
		assert.NoError(t, err)

		zosZmachine, ok := dataI.(*zos.ZMachine)
		assert.Equal(t, ok, true)

		mountsOfVMWorkload := mounts(zosZmachine.Mounts)
		assert.Equal(t, mountsOfVMWorkload, vm.Mounts)
	})

	t.Run("test_workloads_from_vm", func(t *testing.T) {
		// put state in zero values
		pubIPWorkload.Result.State = ""

		vmWorkloadCp := vmWorkload
		vmWorkloadCp.Result = gridtypes.Result{}

		assert.Equal(t, pubIPWorkload, vm.GenerateVMWorkload()[0])
		assert.Equal(t, vmWorkloadCp, vm.GenerateVMWorkload()[2])
	})

	t.Run("test_vm_validate", func(t *testing.T) {
		assert.NoError(t, vm.Validate())
	})

	t.Run("test_vm_failed_validate", func(t *testing.T) {
		vm.CPU = 0
		assert.ErrorIs(t, vm.Validate(), ErrInvalidInput)
		vm.CPU = 2
	})

	t.Run("test_workload_from_vm", func(t *testing.T) {
		var err error

		workloadsFromVM, err = vm.GenerateWorkloads()
		assert.NoError(t, err)

		vmWorkloadCp := vmWorkload
		vmWorkloadCp.Result = gridtypes.Result{}

		assert.Equal(t, workloadsFromVM[2], vmWorkloadCp)
		assert.Equal(t, workloadsFromVM[0], pubIPWorkload)
	})

	t.Run("test_vm_set_network_name", func(t *testing.T) {
		vmWithNetwork := vm.WithNetworkName("net")
		assert.Equal(t, vmWithNetwork.NetworkName, "net")
	})

	t.Run("test_vm_matches_another_vm", func(t *testing.T) {
		vm2 := vm.WithNetworkName("net")
		vm.Match(vm2)
		assert.Equal(t, *vm2, vm)

		// reset network name
		vm2 = vm.WithNetworkName("test network")
		vm.Match(vm2)
		assert.Equal(t, *vm2, vm)
	})

	t.Run("test_workloads_map", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], workloadsFromVM...)

		workloadsMap2, err := vm.BindWorkloadsToNode(nodeID)
		assert.NoError(t, err)
		assert.Equal(t, workloadsMap, workloadsMap2)
	})
}
