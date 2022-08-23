package workloads

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type VM struct {
	Name          string
	Flist         string
	FlistChecksum string
	PublicIP      bool
	PublicIP6     bool
	Planetary     bool
	Corex         bool
	ComputedIP    string
	ComputedIP6   string
	YggIP         string
	IP            string
	Description   string
	Cpu           int
	Memory        int
	RootfsSize    int
	Entrypoint    string
	Mounts        []Mount
	Zlogs         []Zlog
	EnvVars       map[string]string
	NetworkName   string
}

type Mount struct {
	DiskName   string
	MountPoint string
}

type Zlog struct {
	Output string
}

func (v VM)GenerateWorkloadFromVM(vm VM) ([]gridtypes.Workload, error) {
	workloads := make([]gridtypes.Workload, 0)
	publicIPName := ""
	if vm.PublicIP || vm.PublicIP6 {
		publicIPName = fmt.Sprintf("%sip", vm.Name)
		workloads = append(workloads, constructPublicIPWorkload(publicIPName, vm.PublicIP, vm.PublicIP6))
	}
	mounts := make([]zos.MachineMount, 0)
	for _, mount := range vm.Mounts {
		mounts = append(mounts, zos.MachineMount{Name: gridtypes.Name(mount.DiskName), Mountpoint: mount.MountPoint})
	}
	for _, zlog := range vm.Zlogs {
		workloads = append(workloads, zlog.GenerateWorkload(vm.Name))
	}

	workload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(vm.Name),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: vm.Flist,
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name(vm.NetworkName),
						IP:      net.ParseIP(vm.IP),
					},
				},
				PublicIP:  gridtypes.Name(publicIPName),
				Planetary: vm.Planetary,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(vm.Cpu),
				Memory: gridtypes.Unit(uint(vm.Memory)) * gridtypes.Megabyte,
			},
			Size:       gridtypes.Unit(vm.RootfsSize) * gridtypes.Megabyte,
			Entrypoint: vm.Entrypoint,
			Corex:      vm.Corex,
			Mounts:     mounts,
			Env:        vm.EnvVars,
		}),
		Description: vm.Description,
	}
	workloads = append(workloads, workload)
	return workloads, nil
}

func (v VM) Stage(manager deployer.DeploymentManager, NodeId uint32) error {
	workloadsMap := map[uint32][]gridtypes.Workload{}
	workloads, err := v.GenerateWorkloadFromVM(v)
	if err != nil {
		return err
	}
	workloadsMap[NodeId] = workloads
	err = manager.SetWorkloads(workloadsMap)
	return err
}

func (zlog *Zlog) GenerateWorkload(zmachine string) gridtypes.Workload {
	url := []byte(zlog.Output)
	urlHash := md5.Sum([]byte(url))
	return gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(hex.EncodeToString(urlHash[:])),
		Type:    zos.ZLogsType,
		Data: gridtypes.MustMarshal(zos.ZLogs{
			ZMachine: gridtypes.Name(zmachine),
			Output:   zlog.Output,
		}),
	}
}

func (v *VM) WithNetworkName(name string) *VM {
	v.NetworkName = name
	return v
}

func constructPublicIPWorkload(workloadName string, ipv4 bool, ipv6 bool) gridtypes.Workload {
	return gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(workloadName),
		Type:    zos.PublicIPType,
		Data: gridtypes.MustMarshal(zos.PublicIP{
			V4: ipv4,
			V6: ipv6,
		}),
	}
}
