package workloads

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type VM struct {
	NodeId        uint32
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

func mounts(mounts []zos.MachineMount) []Mount {
	var res []Mount
	for _, mount := range mounts {
		res = append(res, Mount{
			DiskName:   mount.Name.String(),
			MountPoint: mount.Mountpoint,
		})
	}
	return res
}

func LoadVM(d deployer.DeploymentManager, name string, nodeId uint32) {

}

func NewVMFromWorkloads(wl *gridtypes.Workload, dl *gridtypes.Deployment, nodeId uint32) (VM, error) {
	dataI, err := wl.WorkloadData()
	if err != nil {
		return VM{}, errors.Wrap(err, "failed to get workload data")
	}
	// TODO: check ok?
	data := dataI.(*zos.ZMachine)
	var result zos.ZMachineResult
	log.Printf("%+v\n", wl.Result)
	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return VM{}, errors.Wrap(err, "failed to get vm result")
	}

	pubip := pubIP(dl, data.Network.PublicIP)
	var pubip4, pubip6 = "", ""
	if !pubip.IP.Nil() {
		pubip4 = pubip.IP.String()
	}
	if !pubip.IPv6.Nil() {
		pubip6 = pubip.IPv6.String()
	}
	return VM{
		NodeId:        nodeId,
		Name:          wl.Name.String(),
		Description:   wl.Description,
		Flist:         data.FList,
		FlistChecksum: "",
		PublicIP:      !pubip.IP.Nil(),
		ComputedIP:    pubip4,
		PublicIP6:     !pubip.IPv6.Nil(),
		ComputedIP6:   pubip6,
		Planetary:     result.YggIP != "",
		Corex:         data.Corex,
		YggIP:         result.YggIP,
		IP:            data.Network.Interfaces[0].IP.String(),
		Cpu:           int(data.ComputeCapacity.CPU),
		Memory:        int(data.ComputeCapacity.Memory / gridtypes.Megabyte),
		RootfsSize:    int(data.Size / gridtypes.Megabyte),
		Entrypoint:    data.Entrypoint,
		Mounts:        mounts(data.Mounts),
		Zlogs:         zlogs(dl, wl.Name.String()),
		EnvVars:       data.Env,
		NetworkName:   string(data.Network.Interfaces[0].Network),
	}, nil
}

func zlogs(dl *gridtypes.Deployment, name string) []Zlog {
	var res []Zlog
	for _, wl := range dl.ByType(zos.ZLogsType) {
		if !wl.Result.State.IsOkay() {
			continue
		}
		dataI, err := wl.WorkloadData()
		if err != nil {
			continue
		}
		data := dataI.(*zos.ZLogs)
		if data.ZMachine.String() != name {
			continue
		}
		res = append(res, Zlog{
			Output: data.Output,
		})
	}
	return res
}

func pubIP(dl *gridtypes.Deployment, name gridtypes.Name) zos.PublicIPResult {

	pubIPWl, err := dl.Get(name)
	if err != nil || !pubIPWl.Workload.Result.State.IsOkay() {
		pubIPWl = nil
		return zos.PublicIPResult{}
	}
	var pubIPResult zos.PublicIPResult

	_ = json.Unmarshal(pubIPWl.Result.Data, &pubIPResult)
	return pubIPResult
}

func (v VM) Stage(manager deployer.DeploymentManager) (err error) {
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
		err = manager.SetWorkload(v.NodeId, w)
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
