// Package deployer is the grid deployer
package deployer

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func constructTestNetwork() (workloads.ZNet, gridtypes.Deployment) {

	network := workloads.ZNet{
		Name:        "network",
		Description: "network for testing",
		Nodes:       []uint32{nodeID},
		IPRange: gridtypes.NewIPNet(net.IPNet{
			IP:   net.IPv4(10, 1, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}),
		AddWGAccess: false,
	}

	workload := network.GenerateWorkload(network.NodesIPRange[nodeID], "", uint16(0), []zos.Peer{})

	gridDL := workloads.NewGridDeployment(twinID, []gridtypes.Workload{workload})
	return network, gridDL
}

func constructTestNetworkDeployer(t *testing.T, mock bool) (NetworkDeployer, *mocks.RMBMockClient, *mocks.MockSubstrateExt, *mocks.MockNodeClientGetter) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)

	if mock {
		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl

		tfPluginClient.stateLoader.ncPool = ncPool
		tfPluginClient.stateLoader.substrate = sub
	}

	return NewNetworkDeployer(&tfPluginClient), cl, sub, ncPool
}

func TestNetworkValidate(t *testing.T) {
	znet, _ := constructTestNetwork()
	d, _, _, _ := constructTestNetworkDeployer(t, false)

	znet.IPRange.Mask = net.CIDRMask(20, 32)
	assert.Error(t, d.Validate(context.Background(), &znet))

	znet.IPRange.Mask = net.CIDRMask(16, 32)
	assert.NoError(t, d.Validate(context.Background(), &znet))
}

func TestNetworkGenerateDeployment(t *testing.T) {
	net, networkDl := constructTestNetwork()
	d, cl, sub, ncPool := constructTestNetworkDeployer(t, true)

	d.TFPluginClient.stateLoader.currentNodeNetwork[nodeID] = contractID

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.network.public_config_get", gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.network.interfaces", gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.network.list_wg_ports", gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(nodeID)).
		Return(client.NewNodeClient(twinID, cl), nil).
		AnyTimes()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &net)
	assert.NoError(t, err)

	assert.Equal(t, len(networkDl.Workloads), len(dls[net.Nodes[0]].Workloads))
	//assert.Equal(t, networkDl.Workloads, dls[net.Nodes[0]].Workloads)
}

/*
func TestNetworkSync(t *testing.T) {
	net, _ := constructTestNetwork()
	d, cl, sub, ncPool := constructTestNetworkDeployer(t, true)

	_, networkDl := constructTestNetwork()
	d.TFPluginClient.stateLoader.currentNodeNetwork[nodeID] = contractID

	// invalidate contract
	sub.EXPECT().IsValidContract(net.ContractID).Return(false, nil).AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(sub, uint32(nodeID)).
		Return(client.NewNodeClient(twinID, cl), nil)

	cl.EXPECT().
		Call(gomock.Any(), twinID, "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
			var res *gridtypes.Deployment = result.(*gridtypes.Deployment)
			*res = networkDl
			return nil
		}).AnyTimes()

	dls, err := d.GenerateVersionlessDeployments(context.Background(), &net)
	assert.NoError(t, err)

	gridDl := dls[net.NodeID]
	err = json.NewEncoder(log.Writer()).Encode(gridDl.Workloads)
	assert.NoError(t, err)

	for _, zlog := range gridDl.ByType(zos.ZLogsType) {
		*zlog.Workload = zlog.WithResults(gridtypes.Result{
			State: gridtypes.StateOk,
		})
	}

	for _, disk := range gridDl.ByType(zos.ZMountType) {
		*disk.Workload = disk.WithResults(gridtypes.Result{
			State: gridtypes.StateOk,
		})
	}

	wl, err := gridDl.Get(gridtypes.Name(net.Vms[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZMachineResult{
			IP:    net.Vms[0].IP,
			YggIP: net.Vms[0].YggIP,
		}),
	})

	dataI, err := wl.WorkloadData()
	assert.NoError(t, err)

	data := dataI.(*zos.ZMachine)
	pubIP, err := gridDl.Get(data.Network.PublicIP)
	assert.NoError(t, err)

	*pubIP.Workload = pubIP.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.PublicIPResult{
			IP:   gridtypes.MustParseIPNet(net.Vms[0].ComputedIP),
			IPv6: gridtypes.MustParseIPNet(net.Vms[0].ComputedIP6),
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(net.Vms[1].Name))
	assert.NoError(t, err)

	dataI, err = wl.WorkloadData()
	assert.NoError(t, err)

	data = dataI.(*zos.ZMachine)
	pubIP, err = gridDl.Get(data.Network.PublicIP)
	assert.NoError(t, err)

	*pubIP.Workload = pubIP.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.PublicIPResult{
			IPv6: gridtypes.MustParseIPNet(net.Vms[1].ComputedIP6),
		}),
	})

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZMachineResult{
			IP:    net.Vms[1].IP,
			YggIP: net.Vms[1].YggIP,
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(net.Qsfs[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.QuatumSafeFSResult{
			MetricsEndpoint: net.Qsfs[0].MetricsEndpoint,
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(net.Zdbs[0].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZDBResult{
			Namespace: net.Zdbs[0].Namespace,
			IPs:       net.Zdbs[0].IPs,
			Port:      uint(net.Zdbs[0].Port),
		}),
	})

	wl, err = gridDl.Get(gridtypes.Name(net.Zdbs[1].Name))
	assert.NoError(t, err)

	*wl.Workload = wl.WithResults(gridtypes.Result{
		State: gridtypes.StateOk,
		Data: mustMarshal(t, zos.ZDBResult{
			Namespace: net.Zdbs[1].Namespace,
			IPs:       net.Zdbs[1].IPs,
			Port:      uint(net.Zdbs[1].Port),
		}),
	})

	for i := 0; 2*i < len(gridDl.Workloads); i++ {
		gridDl.Workloads[i], gridDl.Workloads[len(gridDl.Workloads)-1-i] =
			gridDl.Workloads[len(gridDl.Workloads)-1-i], gridDl.Workloads[i]
	}

	sub.EXPECT().IsValidContract(contractID).Return(true, nil)

	var cp workloads.Deployment
	musUnmarshal(t, mustMarshal(t, net), &cp)

	_, err = workloads.GetUsedIPs(gridDl)
	assert.NoError(t, err)

	//manager.EXPECT().Commit(context.Background()).AnyTimes()
	assert.NoError(t, d.Sync(context.Background()))
	assert.Equal(t, net.Vms, cp.Vms)
	assert.Equal(t, net.Disks, cp.Disks)
	assert.Equal(t, net.Qsfs, cp.Qsfs)
	assert.Equal(t, net.Zdbs, cp.Zdbs)
	assert.Equal(t, net.ContractID, cp.ContractID)
	assert.Equal(t, net.NodeID, cp.NodeID)
}
*/
