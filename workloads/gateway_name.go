// Package workloads includes workloads types (vm, zdb, QSFS, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

// GatewayNameProxy struct for gateway name proxy
type GatewayNameProxy struct {
	// Required
	NodeID uint32
	// Name the fully qualified domain name to use (cannot be present with Name)
	Name string
	// Backends are list of backend ips
	Backends []zos.Backend

	// Optional
	// Passthrough whether to pass tls traffic or not
	TLSPassthrough bool
	Description    string
	SolutionType   string

	// computed
	// FQDN deployed on the node
	NodeDeploymentID map[uint32]uint64
	FQDN             string
	NameContractID   uint64
	ContractID       uint64
}

// NewGatewayNameProxyFromZosWorkload generates a gateway name proxy from a zos workload
func NewGatewayNameProxyFromZosWorkload(wl gridtypes.Workload) (GatewayNameProxy, error) {
	var result zos.GatewayProxyResult

	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return GatewayNameProxy{}, errors.Wrap(err, "error unmarshalling json")
	}

	dataI, err := wl.WorkloadData()
	if err != nil {
		return GatewayNameProxy{}, errors.Wrap(err, "failed to get workload data")
	}

	data, ok := dataI.(*zos.GatewayNameProxy)
	if !ok {
		return GatewayNameProxy{}, fmt.Errorf("could not create gateway name proxy workload from data %v", dataI)
	}

	return GatewayNameProxy{
		Name:           data.Name,
		TLSPassthrough: data.TLSPassthrough,
		Backends:       data.Backends,
		FQDN:           result.FQDN,
	}, nil
}

// ZosWorkload generates a zos workload from GatewayNameProxy
func (g *GatewayNameProxy) ZosWorkload() gridtypes.Workload {
	return gridtypes.Workload{
		Version: 0,
		Type:    zos.GatewayNameProxyType,
		Name:    gridtypes.Name(g.Name),
		// REVISE: whether description should be set here
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           g.Name,
			TLSPassthrough: g.TLSPassthrough,
			Backends:       g.Backends,
		}),
	}
}

// GenerateMetadata generates gateway deployment metadata
func (g *GatewayNameProxy) GenerateMetadata() (string, error) {
	if len(g.SolutionType) == 0 {
		g.SolutionType = "Gateway"
	}

	deploymentData := DeploymentData{
		Name:        g.FQDN,
		Type:        "Gateway Name",
		ProjectName: g.SolutionType,
	}

	deploymentDataBytes, err := json.Marshal(deploymentData)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse deployment data %v", deploymentData)
	}

	return string(deploymentDataBytes), nil
}
