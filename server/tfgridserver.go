// Package server for running tfgrid daemon
package server

import "github.com/threefoldtech/grid3-go/workloads"

type TFGridServer interface {
	DeployVM(vm VMParams) error
	DeployName(gatewayName GatewayNameParams) error
	DeployFQDN(gatewayFQDN GatewayFQDNParams) error
	DeployK8s(k8s K8sParams) error
	GetName(name string) (workloads.GatewayNameProxy, error)
	GetFQDN(name string) (workloads.GatewayFQDNProxy, error)
	GetVM(name string) (workloads.VM, error)
	Login(mnemonics, network string) error
	Cancel(name string) error
}

type tfGridServerImpl struct{}

// to verify that tfGridServerImpl implements TFGridServer interface
var _ TFGridServer = (*tfGridServerImpl)(nil)

func (t tfGridServerImpl) DeployVM(vm VMParams) error {
	return nil
}

func (t tfGridServerImpl) DeployName(gatewayName GatewayNameParams) error {
	return nil
}

func (t tfGridServerImpl) DeployFQDN(gatewayFQDN GatewayFQDNParams) error {
	return nil
}

func (t tfGridServerImpl) DeployK8s(k8s K8sParams) error {
	return nil
}

func (t tfGridServerImpl) GetName(name string) (workloads.GatewayNameProxy, error) {
	return workloads.GatewayNameProxy{}, nil
}

func (t tfGridServerImpl) GetFQDN(name string) (workloads.GatewayFQDNProxy, error) {
	return workloads.GatewayFQDNProxy{}, nil
}

func (t tfGridServerImpl) GetVM(name string) (workloads.VM, error) {
	return workloads.VM{}, nil
}

func (t tfGridServerImpl) Login(mnemonics, network string) error {
	return nil
}

func (t tfGridServerImpl) Cancel(name string) error {
	return nil
}
