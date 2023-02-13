// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// K8sNode kubernetes data
type K8sNode struct {
	Name          string
	Node          uint32
	DiskSize      int
	PublicIP      bool
	PublicIP6     bool
	Planetary     bool
	Flist         string
	FlistChecksum string
	ComputedIP    string
	ComputedIP6   string
	YggIP         string
	IP            string
	CPU           int
	Memory        int
}

// K8sCluster struct for k8s cluster
type K8sCluster struct {
	Master      *K8sNode
	Workers     []K8sNode
	Token       string
	NetworkName string

	//optional
	SolutionType string
	SSHKey       string

	//computed
	NodesIPRange     map[uint32]gridtypes.IPNet
	NodeDeploymentID map[uint32]uint64
}

// NewK8sNodeFromMap generates new k8s node
func NewK8sNodeFromMap(m map[string]interface{}) K8sNode {
	return K8sNode{
		Name:          m["name"].(string),
		Node:          uint32(m["node"].(int)),
		DiskSize:      m["disk_size"].(int),
		PublicIP:      m["publicip"].(bool),
		PublicIP6:     m["publicip6"].(bool),
		Planetary:     m["planetary"].(bool),
		Flist:         m["flist"].(string),
		FlistChecksum: m["flist_checksum"].(string),
		ComputedIP:    m["computedip"].(string),
		ComputedIP6:   m["computedip6"].(string),
		YggIP:         m["ygg_ip"].(string),
		IP:            m["ip"].(string),
		CPU:           m["cpu"].(int),
		Memory:        m["memory"].(int),
	}
}

// NewK8sNodeFromWorkload generates a new k8s from a workload
func NewK8sNodeFromWorkload(wl gridtypes.Workload, nodeID uint32, diskSize int, computedIP string, computedIP6 string) (K8sNode, error) {
	var k K8sNode
	data, err := wl.WorkloadData()
	if err != nil {
		return k, err
	}
	d := data.(*zos.ZMachine)
	var result zos.ZMachineResult

	if !reflect.DeepEqual(wl.Result, gridtypes.Result{}) {
		err = wl.Result.Unmarshal(&result)
		if err != nil {
			return k, err
		}
	}

	flistCheckSum, err := GetFlistChecksum(d.FList)
	if err != nil {
		return k, err
	}
	return K8sNode{
		Name:          string(wl.Name),
		Node:          nodeID,
		DiskSize:      diskSize,
		PublicIP:      computedIP != "",
		PublicIP6:     computedIP6 != "",
		Planetary:     result.YggIP != "",
		Flist:         d.FList,
		FlistChecksum: flistCheckSum,
		ComputedIP:    computedIP,
		ComputedIP6:   computedIP6,
		YggIP:         result.YggIP,
		IP:            d.Network.Interfaces[0].IP.String(),
		CPU:           int(d.ComputeCapacity.CPU),
		Memory:        int(d.ComputeCapacity.Memory / gridtypes.Megabyte),
	}, nil
}

// ToMap converts k8s node to a map (dict)
func (k *K8sNode) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"name":           k.Name,
		"node":           int(k.Node),
		"disk_size":      k.DiskSize,
		"publicip":       k.PublicIP,
		"publicip6":      k.PublicIP6,
		"planetary":      k.Planetary,
		"flist":          k.Flist,
		"flist_checksum": k.FlistChecksum,
		"computedip":     k.ComputedIP,
		"computedip6":    k.ComputedIP6,
		"ygg_ip":         k.YggIP,
		"ip":             k.IP,
		"cpu":            k.CPU,
		"memory":         k.Memory,
	}
}

// MasterZosWorkload generates a k8s master workload from a k8s node
func (k *K8sNode) MasterZosWorkload(cluster *K8sCluster) (K8sWorkloads []gridtypes.Workload) {
	return k.zosWorkload(cluster, false)
}

// WorkerZosWorkload generates a k8s worker workload from a k8s node
func (k *K8sNode) WorkerZosWorkload(cluster *K8sCluster) (K8sWorkloads []gridtypes.Workload) {
	return k.zosWorkload(cluster, true)
}

// ZosWorkloads generates k8s workloads from a k8s cluster
func (k *K8sCluster) ZosWorkloads() ([]gridtypes.Workload, error) {
	k8sWorkloads := []gridtypes.Workload{}
	k8sWorkloads = append(k8sWorkloads, k.Master.MasterZosWorkload(k)...)

	for _, worker := range k.Workers {
		k8sWorkloads = append(k8sWorkloads, worker.WorkerZosWorkload(k)...)
	}

	return k8sWorkloads, nil
}

// GenerateMetadata generates deployment metadata
func (k *K8sCluster) GenerateMetadata() (string, error) {
	if len(k.SolutionType) == 0 {
		k.SolutionType = "Kubernetes"
	}

	deploymentData := DeploymentData{
		Name:        k.Master.Name,
		Type:        "kubernetes",
		ProjectName: k.SolutionType,
	}

	deploymentDataBytes, err := json.Marshal(deploymentData)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse deployment data %v", deploymentData)
	}

	return string(deploymentDataBytes), nil
}

// ValidateToken validate cluster token
func (k *K8sCluster) ValidateToken() error {
	if len(k.Token) < 6 {
		return errors.New("token must be at least 6 characters")
	}
	if len(k.Token) > 15 {
		return errors.New("token must be at most 15 characters")
	}

	isAlphanumeric := regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(k.Token)
	if !isAlphanumeric {
		return errors.New("token should be alphanumeric")
	}

	return nil
}

// ValidateIPranges validates NodesIPRange of master && workers of k8s cluster
func (k *K8sCluster) ValidateIPranges() error {
	if _, ok := k.NodesIPRange[k.Master.Node]; !ok {
		return fmt.Errorf("the master node %d doesn't exist in the network's ip ranges", k.Master.Node)
	}
	for _, w := range k.Workers {
		if _, ok := k.NodesIPRange[w.Node]; !ok {
			return fmt.Errorf("the node with id %d in worker %s doesn't exist in the network's ip ranges", w.Node, w.Name)
		}
	}
	return nil
}

// ValidateNames validate names for master and workers
func (k *K8sCluster) ValidateNames() error {
	names := make(map[string]bool)
	names[k.Master.Name] = true

	for _, w := range k.Workers {
		if _, ok := names[w.Name]; ok {
			return fmt.Errorf("k8s workers and master must have unique names: %s occurred more than once", w.Name)
		}
		names[w.Name] = true
	}
	return nil
}

// ValidateChecksums validate check sums for k8s flist
func (k *K8sCluster) ValidateChecksums() error {
	nodes := append(k.Workers, *k.Master)
	for _, vm := range nodes {
		if vm.FlistChecksum == "" {
			continue
		}
		checksum, err := GetFlistChecksum(vm.Flist)
		if err != nil {
			return errors.Wrapf(err, "couldn't get flist %s hash", vm.Flist)
		}
		if vm.FlistChecksum != checksum {
			return fmt.Errorf("passed checksum %s of %s doesn't match %s returned from %s",
				vm.FlistChecksum,
				vm.Name,
				checksum,
				FlistChecksumURL(vm.Flist),
			)
		}
	}
	return nil
}

// InvalidateBrokenAttributes removes outdated attrs and deleted contracts
func (k *K8sCluster) InvalidateBrokenAttributes(sub subi.SubstrateExt) error {
	if len(k.NodeDeploymentID) == 0 {
		return nil
	}

	newWorkers := make([]K8sNode, 0)
	validNodes := make(map[uint32]struct{})
	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.State.IsCreated) || errors.Is(err, substrate.ErrNotFound) {
			delete(k.NodeDeploymentID, node)
			delete(k.NodesIPRange, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		} else {
			validNodes[node] = struct{}{}
		}

	}
	if _, ok := validNodes[k.Master.Node]; !ok {
		k.Master = &K8sNode{}
	}
	for _, worker := range k.Workers {
		if _, ok := validNodes[worker.Node]; ok {
			newWorkers = append(newWorkers, worker)
		}
	}
	k.Workers = newWorkers
	return nil
}

func (k *K8sNode) zosWorkload(cluster *K8sCluster, isWorker bool) (K8sWorkloads []gridtypes.Workload) {
	diskName := fmt.Sprintf("%sdisk", k.Name)
	diskWorkload := gridtypes.Workload{
		Name:        gridtypes.Name(diskName),
		Version:     0,
		Type:        zos.ZMountType,
		Description: "",
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(k.DiskSize) * gridtypes.Gigabyte,
		}),
	}
	K8sWorkloads = append(K8sWorkloads, diskWorkload)
	publicIPName := ""
	if k.PublicIP || k.PublicIP6 {
		publicIPName = fmt.Sprintf("%sip", k.Name)
		K8sWorkloads = append(K8sWorkloads, ConstructPublicIPWorkload(publicIPName, k.PublicIP, k.PublicIP6))
	}
	envVars := map[string]string{
		"SSH_KEY":           cluster.SSHKey,
		"K3S_TOKEN":         cluster.Token,
		"K3S_DATA_DIR":      "/mydisk",
		"K3S_FLANNEL_IFACE": "eth0",
		"K3S_NODE_NAME":     k.Name,
		"K3S_URL":           "",
	}
	if isWorker {
		// K3S_URL marks where to find the master node
		envVars["K3S_URL"] = fmt.Sprintf("https://%s:6443", cluster.Master.IP)
	}
	workload := gridtypes.Workload{
		Version: 0,
		Name:    gridtypes.Name(k.Name),
		Type:    zos.ZMachineType,
		Data: gridtypes.MustMarshal(zos.ZMachine{
			FList: k.Flist,
			Network: zos.MachineNetwork{
				Interfaces: []zos.MachineInterface{
					{
						Network: gridtypes.Name(cluster.NetworkName),
						IP:      net.ParseIP(k.IP),
					},
				},
				PublicIP:  gridtypes.Name(publicIPName),
				Planetary: k.Planetary,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(k.CPU),
				Memory: gridtypes.Unit(uint(k.Memory)) * gridtypes.Megabyte,
			},
			Entrypoint: "/sbin/zinit init",
			Mounts: []zos.MachineMount{
				{Name: gridtypes.Name(diskName), Mountpoint: "/mydisk"},
			},
			Env: envVars,
		}),
	}
	K8sWorkloads = append(K8sWorkloads, workload)

	return K8sWorkloads
}
