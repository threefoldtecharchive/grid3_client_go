package workloads

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type NetworkDeployer struct {
	Name        string
	Description string
	Nodes       []uint32
	IPRange     gridtypes.IPNet
	AddWGAccess bool

	AccessWGConfig   string
	ExternalIP       *gridtypes.IPNet
	ExternalSK       wgtypes.Key
	ExternalPK       wgtypes.Key
	PublicNodeID     uint32
	NodeDeploymentID map[uint32]uint64
	NodesIPRange     map[uint32]gridtypes.IPNet

	WGPort map[uint32]int
	Keys   map[uint32]wgtypes.Key
}

// NewNetworkDeployer generates a new network deployer
func NewNetworkDeployer(manager deployer.DeploymentManager, network ZNet) (NetworkDeployer, error) {
	// externalIP, err := gridtypes.ParseIPNet(userAccess.UserAddress)
	// if err != nil {
	// 	return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse user address")
	// }
	// var secretKey wgtypes.Key
	// if userSecretKey == "" {
	// 	secretKey, err := wgtypes.GeneratePrivateKey()
	// 	if err != nil {
	// 		return NetworkDeployer{}, errors.Wrapf(err, "couldn't generate new private key")
	// 	}
	// } else {
	// 	secretKey, err := wgtypes.ParseKey(userSecretKey)
	// 	if err != nil {
	// 		return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse private key %s", userSecretKey)
	// 	}
	// }

	k := NetworkDeployer{
		Name:             network.Name,
		Description:      network.Description,
		Nodes:            network.Nodes,
		IPRange:          network.IPRange,
		AddWGAccess:      network.AddWGAccess,
		ExternalIP:       nil,
		ExternalSK:       wgtypes.Key{},
		ExternalPK:       wgtypes.Key{},
		AccessWGConfig:   "",
		PublicNodeID:     0,
		NodeDeploymentID: make(map[uint32]uint64),
		NodesIPRange:     make(map[uint32]gridtypes.IPNet),
		WGPort:           make(map[uint32]int),
		Keys:             make(map[uint32]wgtypes.Key),
	}

	peerSubnets := map[string]*zos.Peer{}
	nodeSubnets := map[string]bool{}
	// retrieve last network state
	oldDeployments := map[uint32]uint64{}
	for k, v := range manager.GetContractIDs() {
		oldDeployments[k] = v
	}
	for _, nodeID := range k.Nodes {

		if _, ok := oldDeployments[nodeID]; !ok {
			// if node is new, it has no previous state and shouldn't be processed
			continue
		}
		delete(oldDeployments, nodeID)

		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment with nodeID %d", nodeID)
		}
		for _, wl := range dl.Workloads {
			if wl.Name.String() == network.Name && wl.Result.State == gridtypes.StateOk {
				dataI, err := wl.WorkloadData()
				if err != nil {
					return NetworkDeployer{}, errors.Wrapf(err, "couldn't get workload \"%s\" data", wl.Name.String())
				}
				data, ok := dataI.(*zos.Network)
				if !ok {
					return NetworkDeployer{}, errors.New("couldn't cast workload data")
				}
				privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
				if err != nil {
					return NetworkDeployer{}, errors.Wrap(err, "couldn't parse private key")
				}
				for idx, peer := range data.Peers {
					peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
					if k.AddWGAccess && peer.Endpoint == "" {
						// this is the access node
						k.PublicNodeID = nodeID
					}
				}
				nodeSubnets[data.Subnet.String()] = true

				k.NodeDeploymentID[nodeID] = dl.ContractID
				k.Keys[nodeID] = privateKey
				k.WGPort[nodeID] = int(data.WGListenPort)
				k.NodesIPRange[nodeID] = data.Subnet
			}
		}

	}
	// TODO: if oldDeployments is not empty and has any of this networks' workloads, they should be canceled
	// if the workload represents an access node, and the user requires wg access, the workload should not be cancelled
	toCancel := map[uint32]map[string]bool{}
	for nodeID, contractID := range oldDeployments {
		dl, err := manager.GetDeployment(nodeID)
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't get deployment %d", contractID)
		}
		wlID, err := dl.Get(gridtypes.Name(network.Name))

		if err == nil {
			wl := *(wlID.Workload)
			dataI, err := wl.WorkloadData()
			if err != nil {
				return NetworkDeployer{}, errors.Wrap(err, "failed to get workload data")
			}
			data, ok := dataI.(*zos.Network)
			if !ok {
				return NetworkDeployer{}, errors.New("couldn't cast workload data")
			}
			privateKey, err := wgtypes.ParseKey(data.WGPrivateKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrap(err, "couldn't parse private key")
			}

			for idx, peer := range data.Peers {
				peerSubnets[peer.Subnet.String()] = &data.Peers[idx]
				if k.AddWGAccess && peer.Endpoint == "" {
					// this is the access node
					k.PublicNodeID = nodeID
					k.NodeDeploymentID[nodeID] = dl.ContractID
					k.Keys[nodeID] = privateKey
					k.WGPort[nodeID] = int(data.WGListenPort)
					k.NodesIPRange[nodeID] = data.Subnet
					break
				}
			}
			nodeSubnets[data.Subnet.String()] = true

			if !k.AddWGAccess || k.PublicNodeID != nodeID {
				// this is a node to be cancelled
				toCancel[nodeID] = make(map[string]bool)
				toCancel[nodeID][network.Name] = true
			}

		}
	}
	for subnet, peer := range peerSubnets {
		if _, ok := nodeSubnets[subnet]; !ok {
			// this was the user access ip
			externalIP, err := gridtypes.ParseIPNet(subnet)
			if err != nil {
				return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse user address")
			}
			k.ExternalIP = &externalIP
			pk, err := wgtypes.ParseKey(peer.WGPublicKey)
			if err != nil {
				return NetworkDeployer{}, errors.Wrapf(err, "couldn't parse peer wg public key")
			}
			k.ExternalPK = pk
			break
		}
	}

	if k.ExternalIP == nil {
		// user does not have userAccess configs and a new private key should be generated
		secretKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return NetworkDeployer{}, errors.Wrapf(err, "couldn't generate new private key")
		}
		k.ExternalSK = secretKey
		k.ExternalPK = secretKey.PublicKey()
	}

	err := manager.CancelWorkloads(toCancel)
	if err != nil {
		return NetworkDeployer{}, errors.Wrapf(err, "couldn't cancel workloads")
	}

	return k, nil
}

func nextFreeOctet(used []byte, start *byte) error {
	for Contains(used, *start) && *start <= 254 {
		*start += 1
	}
	if *start == 255 {
		return errors.New("couldn't find a free ip to add node")
	}
	return nil
}

func (k *NetworkDeployer) assignNodesIPs(nodes []uint32) error {
	ips := make(map[uint32]gridtypes.IPNet)
	l := len(k.IPRange.IP)
	usedIPs := make([]byte, 0) // the third octet
	for node, ip := range k.NodesIPRange {
		if Contains(nodes, node) {
			usedIPs = append(usedIPs, ip.IP[l-2])
			ips[node] = ip
		}
	}
	var cur byte = 2
	if k.AddWGAccess {
		if k.ExternalIP != nil {
			usedIPs = append(usedIPs, k.ExternalIP.IP[l-2])
		} else {
			err := nextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ip := IpNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-1], 24)
			k.ExternalIP = &ip
		}
	}
	for _, node := range nodes {
		if _, ok := ips[node]; !ok {
			err := nextFreeOctet(usedIPs, &cur)
			if err != nil {
				return err
			}
			usedIPs = append(usedIPs, cur)
			ips[node] = IpNet(k.IPRange.IP[l-4], k.IPRange.IP[l-3], cur, k.IPRange.IP[l-2], 24)
		}
	}
	k.NodesIPRange = ips
	return nil
}
func (k *NetworkDeployer) assignNodesWGPort(ctx context.Context, sub subi.SubstrateExt, nodes []uint32, ncPool *client.NodeClientPool) error {
	for _, node := range nodes {
		if _, ok := k.WGPort[node]; !ok {
			cl, err := ncPool.GetNodeClient(sub, node)
			if err != nil {
				return errors.Wrap(err, "coudln't get node client")
			}
			port, err := GetNodeFreeWGPort(ctx, cl, node)
			if err != nil {
				return errors.Wrap(err, "failed to get node free wg ports")
			}
			k.WGPort[node] = port
		}
	}

	return nil
}
func (k *NetworkDeployer) assignNodesWGKey(nodes []uint32) error {
	for _, node := range nodes {
		if _, ok := k.Keys[node]; !ok {

			key, err := wgtypes.GenerateKey()
			if err != nil {
				return errors.Wrap(err, "failed to generate wg private key")
			}
			k.Keys[node] = key
		}
	}

	return nil
}

func (k *NetworkDeployer) Validate(ctx context.Context, sub subi.SubstrateExt, identity subi.Identity, ncPool *client.NodeClientPool) error {
	if err := validateAccountBalanceForExtrinsics(sub, identity); err != nil {
		return err
	}
	mask := k.IPRange.Mask
	if ones, _ := mask.Size(); ones != 16 {
		return fmt.Errorf("subnet in iprange %s should be 16", k.IPRange.String())
	}

	return client.AreNodesUp(ctx, sub, k.Nodes, ncPool)
}

func validateAccountBalanceForExtrinsics(sub subi.SubstrateExt, identity subi.Identity) error {
	acc, err := sub.GetAccount(identity)
	if err != nil && !errors.Is(err, substrate.ErrAccountNotFound) {
		return errors.Wrap(err, "failed to get account with the given mnemonics")
	}
	log.Printf("money %d\n", acc.Data.Free)
	if acc.Data.Free.Cmp(big.NewInt(20000)) == -1 {
		return fmt.Errorf("account workloads.Contains %s, min fee is 20000", acc.Data.Free)
	}
	return nil
}

func (k *NetworkDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt, ncPool *client.NodeClientPool) error {

	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, substrate.ErrNotFound) {
			delete(k.NodeDeploymentID, node)
			delete(k.NodesIPRange, node)
			delete(k.Keys, node)
			delete(k.WGPort, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		}
	}
	if k.ExternalIP != nil && !k.IPRange.Contains(k.ExternalIP.IP) {
		k.ExternalIP = nil
	}
	for node, ip := range k.NodesIPRange {
		if !k.IPRange.Contains(ip.IP) {
			delete(k.NodesIPRange, node)
		}
	}
	if k.PublicNodeID != 0 {
		// TODO: add a check that the node is still public
		cl, err := ncPool.GetNodeClient(sub, k.PublicNodeID)
		if err != nil {
			// whatever the error, delete it and it will get reassigned later
			k.PublicNodeID = 0
		}
		if err := cl.IsNodeUp(context.Background()); err != nil {
			k.PublicNodeID = 0
		}
	}

	if !k.AddWGAccess {
		k.ExternalIP = nil
	}
	return nil
}
