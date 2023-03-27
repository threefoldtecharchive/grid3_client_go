// Package deployer for grid deployer
package deployer

import (
	"context"
	"math/big"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/grid3-go/mocks"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

func constructTestK8s(t *testing.T, mock bool) (
	K8sDeployer,
	*mocks.RMBMockClient,
	*mocks.MockSubstrateExt,
	*mocks.MockNodeClientGetter,
	*mocks.MockDeployer,
	*mocks.MockClient,
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tfPluginClient, err := setup()
	assert.NoError(t, err)

	cl := mocks.NewRMBMockClient(ctrl)
	sub := mocks.NewMockSubstrateExt(ctrl)
	ncPool := mocks.NewMockNodeClientGetter(ctrl)
	deployer := mocks.NewMockDeployer(ctrl)
	gridProxyCl := mocks.NewMockClient(ctrl)

	if mock {
		tfPluginClient.twinID = twinID

		tfPluginClient.SubstrateConn = sub
		tfPluginClient.NcPool = ncPool
		tfPluginClient.RMB = cl
		tfPluginClient.GridProxyClient = gridProxyCl

		tfPluginClient.State.ncPool = ncPool
		tfPluginClient.State.substrate = sub

		tfPluginClient.K8sDeployer.deployer = deployer
		tfPluginClient.K8sDeployer.tfPluginClient = &tfPluginClient
	}
	net := constructTestNetwork()
	tfPluginClient.State.networks = networkState{net.Name: network{
		subnets:               map[uint32]string{nodeID: net.IPRange.String()},
		nodeDeploymentHostIDs: map[uint32]deploymentHostIDs{nodeID: map[uint64][]byte{contractID: {}}},
	}}

	return tfPluginClient.K8sDeployer, cl, sub, ncPool, deployer, gridProxyCl
}

func k8sMockValidation(identity substrate.Identity, cl *mocks.RMBMockClient, sub *mocks.MockSubstrateExt, ncPool *mocks.MockNodeClientGetter, proxyCl *mocks.MockClient, d K8sDeployer) {
	sub.EXPECT().
		GetBalance(d.tfPluginClient.Identity).
		Return(substrate.Balance{
			Free: types.U128{
				Int: big.NewInt(100000),
			},
		}, nil)

	cl.EXPECT().
		Call(
			gomock.Any(),
			nodeID,
			"zos.system.version",
			nil,
			gomock.Any(),
		).Return(nil).AnyTimes()

	ncPool.EXPECT().
		GetNodeClient(
			gomock.Any(),
			nodeID,
		).Return(client.NewNodeClient(nodeID, cl, d.tfPluginClient.rmbTimeout), nil)

}

func constructK8sCluster() (workloads.K8sCluster, error) {
	flist := "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist"
	flistCheckSum, err := workloads.GetFlistChecksum(flist)
	if err != nil {
		return workloads.K8sCluster{}, err
	}

	master := workloads.K8sNode{
		Name:          "K8sForTesting",
		Node:          nodeID,
		DiskSize:      5,
		PublicIP:      true,
		PublicIP6:     true,
		Planetary:     true,
		Flist:         flist,
		FlistChecksum: flistCheckSum,
		ComputedIP:    "5.5.5.5/24",
		ComputedIP6:   "::7/64",
		YggIP:         "::8/64",
		IP:            "10.1.0.2",
		CPU:           2,
		Memory:        1024,
	}

	worker := workloads.K8sNode{
		Name:          "worker1",
		Node:          nodeID,
		DiskSize:      5,
		PublicIP:      true,
		PublicIP6:     true,
		Planetary:     true,
		Flist:         flist,
		FlistChecksum: flistCheckSum,
		ComputedIP:    "",
		ComputedIP6:   "",
		YggIP:         "",
		IP:            "",
		CPU:           2,
		Memory:        1024,
	}
	workers := []workloads.K8sNode{worker}
	Cluster := workloads.K8sCluster{
		Master:       &master,
		Workers:      workers[:],
		Token:        "tokens",
		SSHKey:       "",
		NetworkName:  "network",
		NodesIPRange: make(map[uint32]gridtypes.IPNet),
	}
	return Cluster, nil
}

func TestK8sDeployer(t *testing.T) {
	d, cl, sub, ncPool, deployer, proxyCl := constructTestK8s(t, true)
	k8sCluster, err := constructK8sCluster()
	assert.NoError(t, err)

	t.Run("test validate master reachable", func(t *testing.T) {
		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		err = d.assignNodeIPRange(&k8sCluster)
		assert.NoError(t, err)

		err = d.Validate(context.Background(), &k8sCluster)
		assert.NoError(t, err)
	})

	t.Run("test generate", func(t *testing.T) {
		err = d.assignNodeIPRange(&k8sCluster)
		assert.NoError(t, err)

		dls, err := d.GenerateVersionlessDeployments(context.Background(), &k8sCluster)
		assert.NoError(t, err)

		nodeWorkloads := make(map[uint32][]gridtypes.Workload)
		masterWorkloads := k8sCluster.Master.MasterZosWorkload(&k8sCluster)
		nodeWorkloads[k8sCluster.Master.Node] = append(nodeWorkloads[k8sCluster.Master.Node], masterWorkloads...)
		for _, w := range k8sCluster.Workers {
			workerWorkloads := w.WorkerZosWorkload(&k8sCluster)
			nodeWorkloads[w.Node] = append(nodeWorkloads[w.Node], workerWorkloads...)
		}

		wl := nodeWorkloads[nodeID]
		testDl := workloads.NewGridDeployment(d.tfPluginClient.twinID, wl)
		testDl.Metadata = "{\"type\":\"kubernetes\",\"name\":\"K8sForTesting\",\"projectName\":\"Kubernetes\"}"

		assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
			nodeID: testDl,
		})
	})

	t.Run("test deploy", func(t *testing.T) {
		err = d.assignNodeIPRange(&k8sCluster)
		assert.NoError(t, err)

		dls, err := d.GenerateVersionlessDeployments(context.Background(), &k8sCluster)
		assert.NoError(t, err)

		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		newDeploymentsSolutionProvider := make(map[uint32]*uint64)
		newDeploymentsSolutionProvider[k8sCluster.Master.Node] = nil

		deployer.EXPECT().Deploy(
			gomock.Any(),
			k8sCluster.NodeDeploymentID,
			dls,
			newDeploymentsSolutionProvider,
		).Return(map[uint32]uint64{nodeID: contractID}, nil)

		err = d.Deploy(context.Background(), &k8sCluster)
		assert.NoError(t, err)

		assert.Equal(t, k8sCluster.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
	})

	t.Run("test update", func(t *testing.T) {
		k8sCluster.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

		err = d.assignNodeIPRange(&k8sCluster)
		assert.NoError(t, err)

		dls, err := d.GenerateVersionlessDeployments(context.Background(), &k8sCluster)
		assert.NoError(t, err)

		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		sub.EXPECT().GetContract(uint64(100)).Return(subi.Contract{
			Contract: &substrate.Contract{
				State: substrate.ContractState{IsCreated: true},
				ContractType: substrate.ContractType{
					NodeContract: substrate.NodeContract{
						Node:           types.U32(nodeID),
						PublicIPsCount: 0,
					},
				}},
		}, nil)

		deployer.EXPECT().Deploy(
			gomock.Any(),
			map[uint32]uint64{nodeID: contractID},
			dls,
			gomock.Any(),
		).Return(map[uint32]uint64{nodeID: contractID}, nil)

		err = d.Deploy(context.Background(), &k8sCluster)
		assert.NoError(t, err)
		assert.Equal(t, k8sCluster.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, d.tfPluginClient.State.currentNodeDeployments, map[uint32]contractIDs{nodeID: {contractID}})
	})

	t.Run("test update failed", func(t *testing.T) {
		d.tfPluginClient.State.currentNodeDeployments = map[uint32]contractIDs{nodeID: {contractID}}
		k8sCluster.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}

		err = d.assignNodeIPRange(&k8sCluster)
		assert.NoError(t, err)

		dls, err := d.GenerateVersionlessDeployments(context.Background(), &k8sCluster)
		assert.NoError(t, err)

		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		sub.EXPECT().GetContract(uint64(100)).Return(subi.Contract{
			Contract: &substrate.Contract{
				State: substrate.ContractState{IsCreated: true},
				ContractType: substrate.ContractType{
					NodeContract: substrate.NodeContract{
						Node:           types.U32(nodeID),
						PublicIPsCount: 0,
					},
				}},
		}, nil)

		deployer.EXPECT().Deploy(
			gomock.Any(),
			map[uint32]uint64{nodeID: contractID},
			dls,
			gomock.Any(),
		).Return(map[uint32]uint64{nodeID: contractID}, errors.New("error"))

		err = d.Deploy(context.Background(), &k8sCluster)
		assert.Error(t, err)
		assert.Equal(t, k8sCluster.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, d.tfPluginClient.State.currentNodeDeployments, map[uint32]contractIDs{nodeID: {contractID}})
	})

	t.Run("test cancel", func(t *testing.T) {
		d.tfPluginClient.State.currentNodeDeployments = map[uint32]contractIDs{nodeID: {contractID}}
		k8sCluster.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
		k8sCluster.NodesIPRange = map[uint32]gridtypes.IPNet{uint32(10): {}}

		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		deployer.EXPECT().Cancel(
			gomock.Any(), contractID,
		).Return(nil)

		err = d.Cancel(context.Background(), &k8sCluster)
		assert.NoError(t, err)
		assert.Empty(t, k8sCluster.NodeDeploymentID)
		assert.Empty(t, d.tfPluginClient.State.currentNodeDeployments[nodeID])
	})

	t.Run("test cancel failed", func(t *testing.T) {
		d.tfPluginClient.State.currentNodeDeployments = map[uint32]contractIDs{nodeID: {contractID}}
		k8sCluster.NodeDeploymentID = map[uint32]uint64{nodeID: contractID}
		k8sCluster.NodesIPRange = map[uint32]gridtypes.IPNet{uint32(10): {}}

		k8sMockValidation(d.tfPluginClient.Identity, cl, sub, ncPool, proxyCl, d)

		deployer.EXPECT().Cancel(
			gomock.Any(), contractID,
		).Return(errors.New("error"))

		err = d.Cancel(context.Background(), &k8sCluster)
		assert.Error(t, err)
		assert.Equal(t, k8sCluster.NodeDeploymentID, map[uint32]uint64{nodeID: contractID})
		assert.Equal(t, d.tfPluginClient.State.currentNodeDeployments, map[uint32]contractIDs{nodeID: {contractID}})
	})
}
