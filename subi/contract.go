package subi

import (
	"github.com/threefoldtech/substrate-client"
)

// Contract is a contract interface
type Contract interface {
	IsDeleted() bool
	IsCreated() bool
	TwinID() uint32
	PublicIPCount() uint32
}

// ContractImpl is for contract implementation
type ContractImpl struct {
	*substrate.Contract
}

// IsDeleted checks if contract is deleted
func (c *ContractImpl) IsDeleted() bool {
	return c.Contract.State.IsDeleted
}

// IsCreated checks if contract is created
func (c *ContractImpl) IsCreated() bool {
	return c.Contract.State.IsCreated
}

// TwinID returns contract's twin ID
func (c *ContractImpl) TwinID() uint32 {
	return uint32(c.Contract.TwinID)
}

// PublicIPCount returns contract's public IPs count
func (c *ContractImpl) PublicIPCount() uint32 {
	return uint32(c.Contract.ContractType.NodeContract.PublicIPsCount)
}
