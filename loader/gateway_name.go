package loader

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func GatewayNameProxyFromZosWorkload(manager deployer.DeploymentManager, nodeID uint32, name string) (workloads.GatewayNameProxy, error) {
	wl, err := manager.GetWorkload(nodeID, name)
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrapf(err, "couldn't get workload from node %d", nodeID)
	}

	var result zos.GatewayProxyResult

	if err := json.Unmarshal(wl.Result.Data, &result); err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrap(err, "error unmarshalling json")
	}
	dataI, err := wl.WorkloadData()
	if err != nil {
		return workloads.GatewayNameProxy{}, errors.Wrap(err, "failed to get workload data")
	}
	data := dataI.(*zos.GatewayNameProxy)

	return workloads.GatewayNameProxy{
		Name:           data.Name,
		TLSPassthrough: data.TLSPassthrough,
		Backends:       data.Backends,
		FQDN:           result.FQDN,
	}, nil

}
