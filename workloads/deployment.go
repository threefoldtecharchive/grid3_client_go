// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"log"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

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
	Qsfs             []QSFS

	// computed
	ContractID uint64
}

func NewDeployment(name string, nodeID uint32,
	solutionType string, solutionProvider *uint64,
	NetworkName string,
	disks []Disk,
	zdbs []ZDB,
	vms []VM,
	qsfs []QSFS,
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
		Qsfs:             qsfs,
		ContractID:       0,
	}
}

// Validate validates a deployment
// TODO: are there any more validations on workloads needed other than vm and network name relation?
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

// Nullify resets deployment
func (d *Deployment) Nullify() {
	d.Vms = nil
	d.Qsfs = nil
	d.Disks = nil
	d.Zdbs = nil
	d.ContractID = 0
}

// Match objects to match the input
func (d *Deployment) Match(disks []Disk, qsfs []QSFS, zdbs []ZDB, vms []VM) {
	vmMap := make(map[string]*VM)
	l := len(d.Disks) + len(d.Qsfs) + len(d.Zdbs) + len(d.Vms)
	names := make(map[string]int)
	for idx, o := range d.Disks {
		names[o.Name] = idx - l
	}
	for idx, o := range d.Qsfs {
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
	sort.Slice(qsfs, func(i, j int) bool {
		return names[qsfs[i].Name] < names[qsfs[j].Name]
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
			vms[idx].Match(vm)
			log.Printf("orig: %+v\n", vm)
			log.Printf("new: %+v\n", vms[idx])
		}
	}
}

// NewDeployment generates a new deployment
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

// GatewayWorkloadGenerator is an interface for a gateway workload generator
type GatewayWorkloadGenerator interface {
	ZosWorkload() gridtypes.Workload
}

// NewDeploymentWithGateway generates a new deployment with a gateway workload
func NewDeploymentWithGateway(identity substrate.Identity, twinID uint32, version uint32, gw GatewayWorkloadGenerator) (gridtypes.Deployment, error) {
	dl := NewGridDeployment(twinID, []gridtypes.Workload{})
	dl.Version = version

	dl.Workloads = append(dl.Workloads, gw.ZosWorkload())
	dl.Workloads[0].Version = version

	err := dl.Sign(twinID, identity)
	if err != nil {
		return gridtypes.Deployment{}, err
	}

	return dl, nil
}
