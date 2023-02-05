// Package subi for substrate client
package subi

import (
	"github.com/threefoldtech/substrate-client"
)

// Contract is for contract implementation
type Contract struct {
	*substrate.Contract
}

// IsDeleted checks if contract is deleted
func (c *Contract) IsDeleted() bool {
	return c.Contract.State.IsDeleted
}

// IsCreated checks if contract is created
func (c *Contract) IsCreated() bool {
	return c.Contract.State.IsCreated
}

// TwinID returns contract's twin ID
func (c *Contract) TwinID() uint32 {
	return uint32(c.Contract.TwinID)
}

// PublicIPCount returns contract's public IPs count
func (c *Contract) PublicIPCount() uint32 {
	return uint32(c.Contract.ContractType.NodeContract.PublicIPsCount)
}
