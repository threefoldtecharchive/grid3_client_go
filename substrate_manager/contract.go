package substratemanager

import (
	client "github.com/threefoldtech/substrate-client"
)

type Contract interface {
	IsDeleted() bool
	IsCreated() bool
	TwinID() uint32
	PublicIPCount() uint32
}

type ContractImpl struct {
	*client.Contract
}

func (c *ContractImpl) IsDeleted() bool {
	return c.Contract.State.IsDeleted
}
func (c *ContractImpl) IsCreated() bool {
	return c.Contract.State.IsCreated
}

func (c *ContractImpl) TwinID() uint32 {
	return uint32(c.Contract.TwinID)
}

func (c *ContractImpl) PublicIPCount() uint32 {
	return uint32(c.Contract.ContractType.NodeContract.PublicIPsCount)
}
