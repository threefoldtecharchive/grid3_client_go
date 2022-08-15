package loader

import (
	"context"
	"encoding/json"
	"log"

	"github.com/pkg/errors"
	deployer "github.com/threefoldtech/grid3-go/deployer/manager"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func LoadVmFromGrid(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.VM, error) {
	dl := gridtypes.Deployment{}
	dM := deployer.deploymentManager{}
	if dID, ok := dM.deploymentIDs[nodeID]; ok {
		s, err := dM.substrate.SubstrateExt()
		if err != nil {
			return workloads.VM{}, errors.Wrapf(err, "couldn't get substrate client")
		}

		defer s.Close()
		nodeClient, err := dM.ncPool.GetNodeClient(s, nodeID)
		if err != nil {
			return workloads.VM{}, errors.Wrapf(err, "couldn't get node client: %d", nodeID)
		}

		dl, err = nodeClient.DeploymentGet(context.Background(), dID)
		if err != nil {
			return workloads.VM{}, errors.Wrapf(err, "couldn't get deployment from node %d", nodeID)
		}
	}

	workload := gridtypes.Workload{}
	dataI, err := workload.WorkloadData()
	if err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to get workload data")
	}

	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.VM{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	data, ok := dataI.(*zos.ZMachine)
	if !ok {
		return workloads.VM{}, errors.New("couldn't cast workload data")
	}
	var result zos.ZMachineResult
	log.Printf("%+v\n", wl.Result)
	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return workloads.VM{}, errors.Wrap(err, "failed to get vm result")
	}

	pubip := pubIP(&dl, data.Network.PublicIP)
	var pubip4, pubip6 = "", ""
	if !pubip.IP.Nil() {
		pubip4 = pubip.IP.String()
	}
	if !pubip.IPv6.Nil() {
		pubip6 = pubip.IPv6.String()
	}

	return workloads.VM{
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
		Zlogs:         zlogs(&dl, wl.Name.String()),
		EnvVars:       data.Env,
		NetworkName:   string(data.Network.Interfaces[0].Network),
	}, nil
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

func mounts(mounts []zos.MachineMount) []workloads.Mount {
	var res []workloads.Mount
	for _, mount := range mounts {
		res = append(res, workloads.Mount{
			DiskName:   mount.Name.String(),
			MountPoint: mount.Mountpoint,
		})
	}
	return res
}

func zlogs(dl *gridtypes.Deployment, name string) []workloads.Zlog {
	var res []workloads.Zlog
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
		res = append(res, workloads.Zlog{
			Output: data.Output,
		})
	}
	return res
}
