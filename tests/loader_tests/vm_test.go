package loader

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/loader"
	mock_deployer "github.com/threefoldtech/grid3-go/tests/mocks"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestLoadVmFromGrid(t *testing.T) {
	vmRes, _ := json.Marshal(zos.ZMachineResult{
		ID:    "5",
		IP:    "5.5.5.5",
		YggIP: "203:8b0b:5f3e:b859:c36:efdf:ab6e:50cc",
	})
	vm := workloads.VM{
		Name:          "test",
		Flist:         "flist test",
		FlistChecksum: "",
		PublicIP:      true,
		ComputedIP:    "189.0.0.12/24",
		PublicIP6:     false,
		Planetary:     true,
		Corex:         false,
		YggIP:         "203:8b0b:5f3e:b859:c36:efdf:ab6e:50cc",
		IP:            "1.1.1.1",
		Description:   "test des",
		Cpu:           2,
		Memory:        2048,
		RootfsSize:    4096,
		Entrypoint:    "entrypoint",
		Mounts: []workloads.Mount{
			{DiskName: "disk", MountPoint: "mount"},
		},
		Zlogs: []workloads.Zlog{
			{Output: "output"},
		},
		EnvVars:     map[string]string{"var1": "val1"},
		NetworkName: "test_network",
	}
	vmWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test"),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: "flist test",
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name("test_network"),
						IP:      net.ParseIP("1.1.1.1"),
					},
				},
				PublicIP:  gridtypes.Name("testip"),
				Planetary: true,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(2),
				Memory: gridtypes.Unit(uint(2048)) * gridtypes.Megabyte,
			},
			Size:       gridtypes.Unit(4096) * gridtypes.Megabyte,
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
	ipRes, _ := json.Marshal(zos.PublicIPResult{
		IP:      gridtypes.MustParseIPNet("189.0.0.12/24"),
		Gateway: nil,
	})
	pubIPWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("testip"),
		Type:    zos.PublicIPType,
		Data: gridtypes.MustMarshal(zos.PublicIP{
			V4: true,
			V6: false,
		}),
		Result: gridtypes.Result{
			Created: 10000,
			State:   gridtypes.StateOk,
			Data:    ipRes,
		},
	}
	zlogWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("test_zlogs"),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name("test"),
			Output:   "output",
		}),
		Result: gridtypes.Result{
			State: gridtypes.StateOk,
		},
	}
	deployment := gridtypes.Deployment{
		Version:     0,
		TwinID:      1,
		ContractID:  100,
		Description: "deployment",
		Workloads: []gridtypes.Workload{
			vmWl,
			pubIPWl,
			zlogWl,
		},
	}
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(vmWl, nil)
		manager.EXPECT().GetDeployment(uint32(1)).Return(deployment, nil)
		got, err := loader.LoadVmFromGrid(manager, 1, "test")
		assert.NoError(t, err)
		assert.Equal(t, vm, got)
	})
	t.Run("invalid type", func(t *testing.T) {
		vmWlCp := vmWl
		vmWlCp.Type = "invalid"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetDeployment(uint32(1)).Return(deployment, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(vmWlCp, nil)

		_, err := loader.LoadVmFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("wrong workload data", func(t *testing.T) {
		vmWlCp := vmWl
		vmWlCp.Type = zos.GatewayFQDNProxyType
		vmWlCp.Data = gridtypes.MustMarshal(zos.GatewayFQDNProxy{
			FQDN: "123",
		})
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetDeployment(uint32(1)).Return(deployment, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(vmWlCp, nil)

		_, err := loader.LoadVmFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
	t.Run("invalid result data", func(t *testing.T) {
		vmWlCp := vmWl
		vmWlCp.Result.Data = nil
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := mock_deployer.NewMockDeploymentManager(ctrl)
		manager.EXPECT().GetDeployment(uint32(1)).Return(deployment, nil)
		manager.EXPECT().GetWorkload(uint32(1), "test").Return(vmWlCp, nil)

		_, err := loader.LoadVmFromGrid(manager, 1, "test")
		assert.Error(t, err)
	})
}
