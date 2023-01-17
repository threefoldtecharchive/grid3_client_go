// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func TestVMStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vm := VM{
		Name:          "test",
		Flist:         "flist test",
		FlistChecksum: "flist cs test",
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
			{Output: "output"},
		},
		EnvVars:     map[string]string{"var1": "val1"},
		NetworkName: "test network",
	}
	pubIPWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name("testip"),
		Type:    zos.PublicIPType,
		Data: gridtypes.MustMarshal(zos.PublicIP{
			V4: true,
			V6: false,
		}),
	}
	urlHash := md5.Sum([]byte("output"))
	zlogWl := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name("test"),
			Output:   "output",
		}),
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
	}

	fmt.Printf("vm: %v\n", vm)

	t.Run("test_set_workloads", func(t *testing.T) {
		nodeID := uint32(1)
		workloadsMap := map[uint32][]gridtypes.Workload{}
		workloadsMap[nodeID] = append(workloadsMap[nodeID], pubIPWl)
		workloadsMap[nodeID] = append(workloadsMap[nodeID], zlogWl)
		workloadsMap[nodeID] = append(workloadsMap[nodeID], vmWl)

		manager := mocks.NewMockDeploymentManager(ctrl)
		manager.EXPECT().SetWorkloads(gomock.Eq(workloadsMap)).Return(nil)

		err := manager.SetWorkloads(workloadsMap)
		assert.NoError(t, err)
	})
}
