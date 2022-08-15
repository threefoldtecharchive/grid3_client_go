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

func (v VM) Stage(manager deployer.DeploymentManager, NodeId uint32) (err error) {
	workloads := make([]gridtypes.Workload, 0)
	publicIPName := ""
	if v.PublicIP || v.PublicIP6 {
		publicIPName = fmt.Sprintf("%sip", v.Name)
		workloads = append(workloads, constructPublicIPWorkload(publicIPName, v.PublicIP, v.PublicIP6))
	}
	mounts := make([]zos.MachineMount, 0)
	for _, mount := range v.Mounts {
		mounts = append(mounts, zos.MachineMount{Name: gridtypes.Name(mount.DiskName), Mountpoint: mount.MountPoint})
	}
	for _, zlog := range v.Zlogs {
		workloads = append(workloads, zlog.GenerateWorkload(v.Name))
	}
	workload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(v.Name),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: v.Flist,
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name(v.NetworkName),
						IP:      net.ParseIP(v.IP),
					},
				},
				PublicIP:  gridtypes.Name(publicIPName),
				Planetary: v.Planetary,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(v.Cpu),
				Memory: gridtypes.Unit(uint(v.Memory)) * gridtypes.Megabyte,
			},
			Size:       gridtypes.Unit(v.RootfsSize) * gridtypes.Megabyte,
			Entrypoint: v.Entrypoint,
			Corex:      v.Corex,
			Mounts:     mounts,
			Env:        v.EnvVars,
		}),
		Description: v.Description,
	}
	workloads = append(workloads, workload)

	for _, w := range workloads {
		err = manager.SetWorkload(NodeId, w)
		if err != nil {
			return err
		}
	}

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
