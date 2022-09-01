package workloads

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

type K8sNodeData struct {
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
	Cpu           int
	Memory        int
}

type K8sDeployer struct {
	Master       *K8sNodeData
	Workers      []K8sNodeData
	NodesIPRange map[uint32]gridtypes.IPNet
	Token        string
	SSHKey       string
	NetworkName  string

	NodeDeploymentID map[uint32]uint64
	UsedIPs          map[uint32][]string
}

type K8sCluster struct {
	Master       *K8sNodeData
	Workers      []K8sNodeData
	NodesIPRange map[uint32]gridtypes.IPNet
	Token        string
	SSHKey       string
	NetworkName  string
}

func (k *K8sDeployer) assignNodesIPs() error {
	// TODO: when a k8s node changes its zos node, remove its ip from the used ones. better at the beginning
	masterNodeRange := k.NodesIPRange[k.Master.Node]
	if k.Master.IP == "" || !masterNodeRange.Contains(net.ParseIP(k.Master.IP)) {
		ip, err := getK8sFreeIP(masterNodeRange, k.UsedIPs[k.Master.Node])
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for master")
		}
		k.Master.IP = ip
		k.UsedIPs[k.Master.Node] = append(k.UsedIPs[k.Master.Node], ip)
	}
	for idx, w := range k.Workers {
		workerNodeRange := k.NodesIPRange[w.Node]
		if w.IP != "" && workerNodeRange.Contains(net.ParseIP(w.IP)) {
			continue
		}
		ip, err := getK8sFreeIP(workerNodeRange, k.UsedIPs[w.Node])
		if err != nil {
			return errors.Wrap(err, "failed to find free ip for worker")
		}
		k.Workers[idx].IP = ip
		k.UsedIPs[w.Node] = append(k.UsedIPs[w.Node], ip)
	}
	return nil
}

func (k *K8sDeployer) Stage(
	ctx context.Context,
	apiClient APIClient,
) error {
	// TODO: check if needed
	err := k.Validate(ctx, apiClient.SubstrateExt, apiClient.Identity, apiClient.NCPool)
	if err != nil {
		return err
	}
	err = k.validateChecksums()
	if err != nil {
		return err
	}

	err = k.invalidateBrokenAttributes(apiClient.SubstrateExt)
	if err != nil {
		return err
	}

	workloads := map[uint32][]gridtypes.Workload{}

	workloads[k.Master.Node] = append(workloads[k.Master.Node], k.Master.GenerateK8sWorkload(apiClient.Manager, k, "")...)
	for _, worker := range k.Workers {
		workloads[k.Master.Node] = append(workloads[k.Master.Node], worker.GenerateK8sWorkload(apiClient.Manager, k, k.Master.IP)...)
	}

	err = k.assignNodesIPs()
	if err != nil {
		return errors.Wrap(err, "failed to assign node ips")
	}

	err = apiClient.Manager.SetWorkloads(workloads)
	if err != nil {
		return err
	}

	return nil
}

func (k *K8sDeployer) invalidateBrokenAttributes(sub subi.SubstrateExt) error {
	newWorkers := make([]K8sNodeData, 0)
	validNodes := make(map[uint32]struct{})
	for node, contractID := range k.NodeDeploymentID {
		contract, err := sub.GetContract(contractID)
		if (err == nil && !contract.IsCreated()) || errors.Is(err, subi.ErrNotFound) {
			delete(k.NodeDeploymentID, node)
			delete(k.NodesIPRange, node)
		} else if err != nil {
			return errors.Wrapf(err, "couldn't get node %d contract %d", node, contractID)
		} else {
			validNodes[node] = struct{}{}
		}

	}
	if _, ok := validNodes[k.Master.Node]; !ok {
		k.Master = &K8sNodeData{}
	}
	for _, worker := range k.Workers {
		if _, ok := validNodes[worker.Node]; ok {
			newWorkers = append(newWorkers, worker)
		}
	}
	k.Workers = newWorkers
	return nil
}

func flistChecksumURL(url string) string {
	return fmt.Sprintf("%s.md5", url)
}

func getFlistChecksum(url string) (string, error) {
	response, err := http.Get(flistChecksumURL(url))
	if err != nil {
		return "", err
	}
	hash, err := ioutil.ReadAll(response.Body)
	return strings.TrimSpace(string(hash)), err
}

func (d *K8sDeployer) validateChecksums() error {
	nodes := append(d.Workers, *d.Master)
	for _, vm := range nodes {
		if vm.FlistChecksum == "" {
			continue
		}
		checksum, err := getFlistChecksum(vm.Flist)
		if err != nil {
			return errors.Wrapf(err, "couldn't get flist %s hash", vm.Flist)
		}
		if vm.FlistChecksum != checksum {
			return fmt.Errorf("passed checksum %s of %s doesn't match %s returned from %s",
				vm.FlistChecksum,
				vm.Name,
				checksum,
				flistChecksumURL(vm.Flist),
			)
		}
	}
	return nil
}

func (k *K8sDeployer) ValidateNames(ctx context.Context) error {

	names := make(map[string]bool)
	names[k.Master.Name] = true
	for _, w := range k.Workers {
		if _, ok := names[w.Name]; ok {
			return fmt.Errorf("k8s workers and master must have unique names: %s occured more than once", w.Name)
		}
		names[w.Name] = true
	}
	return nil
}

func (k *K8sDeployer) ValidateIPranges(ctx context.Context) error {

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

func (k *K8sDeployer) Validate(ctx context.Context, sub subi.SubstrateExt, identity subi.Identity, ncPool *client.NodeClientPool) error {
	if err := validateAccountMoneyForExtrinsics(sub, identity); err != nil {
		return err
	}
	if err := k.ValidateNames(ctx); err != nil {
		return err
	}
	if err := k.ValidateIPranges(ctx); err != nil {
		return err
	}
	nodes := make([]uint32, 0)
	nodes = append(nodes, k.Master.Node)
	for _, w := range k.Workers {
		nodes = append(nodes, w.Node)

	}
	return isNodesUp(ctx, sub, nodes, ncPool)
}

func (k *K8sNodeData) GenerateK8sWorkload(manager deployer.DeploymentManager, deployer *K8sDeployer, masterIP string) []gridtypes.Workload {
	diskName := fmt.Sprintf("%sdisk", k.Name)
	workloads := make([]gridtypes.Workload, 0)
	diskWorkload := gridtypes.Workload{
		Name:        gridtypes.Name(diskName),
		Version:     0,
		Type:        zos.ZMountType,
		Description: "",
		Data: gridtypes.MustMarshal(zos.ZMount{
			Size: gridtypes.Unit(k.DiskSize) * gridtypes.Gigabyte,
		}),
	}
	workloads = append(workloads, diskWorkload)
	publicIPName := ""
	if k.PublicIP || k.PublicIP6 {
		publicIPName = fmt.Sprintf("%sip", k.Name)
		workloads = append(workloads, constructPublicIPWorkload(publicIPName, k.PublicIP, k.PublicIP6))
	}
	envVars := map[string]string{
		"SSH_KEY":           deployer.SSHKey,
		"K3S_TOKEN":         deployer.Token,
		"K3S_DATA_DIR":      "/mydisk",
		"K3S_FLANNEL_IFACE": "eth0",
		"K3S_NODE_NAME":     k.Name,
		"K3S_URL":           "",
	}
	if masterIP != "" {
		envVars["K3S_URL"] = fmt.Sprintf("https://%s:6443", masterIP)
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
						Network: gridtypes.Name(deployer.NetworkName),
						IP:      net.ParseIP(k.IP),
					},
				},
				PublicIP:  gridtypes.Name(publicIPName),
				Planetary: k.Planetary,
			},
			ComputeCapacity: zos.MachineCapacity{
				CPU:    uint8(k.Cpu),
				Memory: gridtypes.Unit(uint(k.Memory)) * gridtypes.Megabyte,
			},
			Entrypoint: "/sbin/zinit init",
			Mounts: []zos.MachineMount{
				{Name: gridtypes.Name(diskName), Mountpoint: "/mydisk"},
			},
			Env: envVars,
		}),
	}
	workloads = append(workloads, workload)
	return workloads
}

func getK8sFreeIP(ipRange gridtypes.IPNet, usedIPs []string) (string, error) {
	i := 254
	l := len(ipRange.IP)
	for i >= 2 {
		ip := ipNet(ipRange.IP[l-4], ipRange.IP[l-3], ipRange.IP[l-2], byte(i), 32)
		ipStr := fmt.Sprintf("%d.%d.%d.%d", ip.IP[l-4], ip.IP[l-3], ip.IP[l-2], ip.IP[l-1])
		log.Printf("ip string: %s\n", ipStr)
		if !isInStr(usedIPs, ipStr) {
			return ipStr, nil
		}
		i -= 1
	}
	return "", errors.New("all ips are used")
}
