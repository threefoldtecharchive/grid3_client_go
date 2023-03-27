// Package workloads includes workloads types (vm, zdb, QSFS, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// DeploymentData for deployments meta data
type DeploymentData struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

// Deployment struct
type Deployment struct {
	Name             string
	NodeID           uint32
	SolutionType     string
	SolutionProvider *uint64
	NetworkName      string
	Disks            []Disk
	Zdbs             []ZDB
	Vms              []VM
	QSFS             []QSFS

	// computed
	NodeDeploymentID map[uint32]uint64
	ContractID       uint64
}

// NewDeployment generates a new deployment
func NewDeployment(name string, nodeID uint32,
	solutionType string, solutionProvider *uint64,
	NetworkName string,
	disks []Disk,
	zdbs []ZDB,
	vms []VM,
	QSFS []QSFS,
) Deployment {
	return Deployment{
		Name:             name,
		NodeID:           nodeID,
		SolutionType:     solutionType,
		SolutionProvider: solutionProvider,
		NetworkName:      NetworkName,
		Disks:            disks,
		Zdbs:             zdbs,
		Vms:              vms,
		QSFS:             QSFS,
	}
}

// Validate validates a deployment
func (d *Deployment) Validate() error {
	if len(d.Vms) != 0 && len(strings.TrimSpace(d.NetworkName)) == 0 {
		return errors.New("if you pass a vm, network name must be non-empty")
	}

	for _, vm := range d.Vms {
		if err := vm.Validate(); err != nil {
			return errors.Wrapf(err, "vm %s validation failed", vm.Name)
		}
	}
	return nil
}

// GenerateMetadata generates deployment metadata
func (d *Deployment) GenerateMetadata() (string, error) {
	if len(d.SolutionType) == 0 {
		d.SolutionType = "Virtual Machine"
	}

	deploymentData := DeploymentData{
		Name:        d.Name,
		Type:        "vm",
		ProjectName: d.SolutionType,
	}

	deploymentDataBytes, err := json.Marshal(deploymentData)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse deployment data %v", deploymentData)
	}

	return string(deploymentDataBytes), nil
}

// Nullify resets deployment
func (d *Deployment) Nullify() {
	d.Vms = nil
	d.QSFS = nil
	d.Disks = nil
	d.Zdbs = nil
	d.ContractID = 0
}

// Match objects to match the input
func (d *Deployment) Match(disks []Disk, QSFS []QSFS, zdbs []ZDB, vms []VM) {
	vmMap := make(map[string]*VM)
	l := len(d.Disks) + len(d.QSFS) + len(d.Zdbs) + len(d.Vms)
	names := make(map[string]int)
	for idx, o := range d.Disks {
		names[o.Name] = idx - l
	}
	for idx, o := range d.QSFS {
		names[o.Name] = idx - l
	}
	for idx, o := range d.Zdbs {
		names[o.Name] = idx - l
	}
	for idx, o := range d.Vms {
		names[o.Name] = idx - l
		vmMap[o.Name] = &d.Vms[idx]
	}
	sort.Slice(disks, func(i, j int) bool {
		return names[disks[i].Name] < names[disks[j].Name]
	})
	sort.Slice(QSFS, func(i, j int) bool {
		return names[QSFS[i].Name] < names[QSFS[j].Name]
	})
	sort.Slice(zdbs, func(i, j int) bool {
		return names[zdbs[i].Name] < names[zdbs[j].Name]
	})
	sort.Slice(vms, func(i, j int) bool {
		return names[vms[i].Name] < names[vms[j].Name]
	})
	for idx := range vms {
		vm, ok := vmMap[vms[idx].Name]
		if ok {
			vms[idx].LoadFromVM(vm)
		}
	}
}

// ZosDeployment generates a new zos deployment from a deployment
func (d *Deployment) ZosDeployment(twin uint32) (gridtypes.Deployment, error) {
	wls := []gridtypes.Workload{}

	for _, d := range d.Disks {
		wls = append(wls, d.ZosWorkload())
	}

	for _, z := range d.Zdbs {
		wls = append(wls, z.ZosWorkload())
	}

	for _, v := range d.Vms {
		vmWls := v.ZosWorkload()
		wls = append(wls, vmWls...)
	}

	for _, q := range d.QSFS {
		qWls, err := q.ZosWorkload()
		if err != nil {
			return gridtypes.Deployment{}, err
		}
		wls = append(wls, qWls)
	}

	return gridtypes.Deployment{
		Version: 0,
		TwinID:  twin, //LocalTwin,
		// this contract id must match the one on substrate
		ContractID: d.ContractID,
		Workloads:  wls,
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: twin,
					Weight: 1,
				},
			},
		},
	}, nil
}

// NewGridDeployment generates a new grid deployment
func NewGridDeployment(twin uint32, workloads []gridtypes.Workload) gridtypes.Deployment {
	return gridtypes.Deployment{
		Version: 0,
		TwinID:  twin, //LocalTwin,
		// this contract id must match the one on substrate
		Workloads: workloads,
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: twin,
					Weight: 1,
				},
			},
		},
	}
}

// GetUsedIPs returns used IPs for a deployment
func GetUsedIPs(dl gridtypes.Deployment) ([]byte, error) {
	usedIPs := []byte{}
	for _, w := range dl.Workloads {
		if !w.Result.State.IsOkay() {
			return usedIPs, fmt.Errorf("workload %s state failed", w.Name)
		}
		if w.Type == zos.ZMachineType {
			vm, err := NewVMFromWorkload(&w, &dl)
			if err != nil {
				return usedIPs, errors.Wrapf(err, "error parsing vm: %s", vm.Name)
			}

			ip := net.ParseIP(vm.IP).To4()
			usedIPs = append(usedIPs, ip[3])
		}
	}
	return usedIPs, nil
}

// ParseDeploymentDate parses the deployment meta date
func ParseDeploymentDate(deploymentMetaData string) (DeploymentData, error) {
	var deploymentData DeploymentData
	err := json.Unmarshal([]byte(deploymentMetaData), &deploymentData)
	if err != nil {
		return DeploymentData{}, err
	}

	return deploymentData, nil
}
