package main

import (
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
)

func main() {
	manager := deployer.NewDeploymentManager()
	vm := workloads.VM{}
	vm.Convert(manager)
}
