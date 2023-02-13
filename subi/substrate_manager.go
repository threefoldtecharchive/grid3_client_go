// Package subi for substrate client
package subi

import (
	"context"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
	"github.com/threefoldtech/substrate-client"
)

// ManagerInterface for substrate manager
type ManagerInterface interface {
	substrate.Manager
	SubstrateExt() (SubstrateImpl, error)
}

// Manager is substrate manager struct
type Manager struct {
	substrate.Manager
	// versioned subdev.Manager
}

// NewManager returns a new substrate manager
func NewManager(url ...string) Manager {
	return Manager{substrate.NewManager(url...)}
}

// SubstrateExt returns a substrate implementation to be used
func (m *Manager) SubstrateExt() (*SubstrateImpl, error) {
	sub, err := m.Manager.Substrate()
	return &SubstrateImpl{sub}, err
}

// SubstrateExt interface for substrate client
type SubstrateExt interface {
	CancelContract(identity substrate.Identity, contractID uint64) error
	CreateNodeContract(identity substrate.Identity, node uint32, body string, hash string, publicIPs uint32, solutionProviderID *uint64) (uint64, error)
	UpdateNodeContract(identity substrate.Identity, contract uint64, body string, hash string) (uint64, error)
	Close()
	GetTwinByPubKey(pk []byte) (uint32, error)

	EnsureContractCanceled(identity substrate.Identity, contractID uint64) error
	DeleteInvalidContracts(contracts map[uint32]uint64) error
	IsValidContract(contractID uint64) (bool, error)
	InvalidateNameContract(
		ctx context.Context,
		identity substrate.Identity,
		contractID uint64,
		name string,
	) (uint64, error)
	GetContract(id uint64) (Contract, error)
	GetNodeTwin(id uint32) (uint32, error)
	CreateNameContract(identity substrate.Identity, name string) (uint64, error)
	GetAccount(identity substrate.Identity) (types.AccountInfo, error)
	GetBalance(identity substrate.Identity) (balance substrate.Balance, err error)
	GetTwinIP(twinID uint32) (string, error)
	GetTwinPK(twinID uint32) ([]byte, error)
	GetContractIDByNameRegistration(name string) (uint64, error)
}

// SubstrateImpl struct to use dev substrate
type SubstrateImpl struct {
	*substrate.Substrate
}

// GetAccount returns the user's account
func (s *SubstrateImpl) GetAccount(identity substrate.Identity) (types.AccountInfo, error) {
	res, err := s.Substrate.GetAccount(identity)
	return res, normalizeNotFoundErrors(err)
}

// GetBalance returns the user's balance
func (s *SubstrateImpl) GetBalance(identity substrate.Identity) (balance substrate.Balance, err error) {
	accountAddress, err := substrate.FromAddress(identity.Address())
	if err != nil {
		return
	}

	balance, err = s.Substrate.GetBalance(accountAddress)
	return balance, normalizeNotFoundErrors(err)
}

// GetNodeTwin returns the twin ID for a node ID
func (s *SubstrateImpl) GetNodeTwin(nodeID uint32) (uint32, error) {
	node, err := s.Substrate.GetNode(nodeID)
	if err != nil {
		return 0, normalizeNotFoundErrors(err)
	}
	return uint32(node.TwinID), nil
}

// GetTwinIP returns twin IP given its ID
func (s *SubstrateImpl) GetTwinIP(id uint32) (string, error) {
	twin, err := s.Substrate.GetTwin(id)
	if err != nil {
		return "", normalizeNotFoundErrors(err)
	}
	return twin.IP, nil
}

// GetTwinPK returns twin's public key
func (s *SubstrateImpl) GetTwinPK(id uint32) ([]byte, error) {
	twin, err := s.Substrate.GetTwin(id)
	if err != nil {
		return nil, normalizeNotFoundErrors(err)
	}
	return twin.Account.PublicKey(), nil
}

// CreateNameContract creates a new name contract
func (s *SubstrateImpl) CreateNameContract(identity substrate.Identity, name string) (uint64, error) {
	return s.Substrate.CreateNameContract(identity, name)
}

// GetContractIDByNameRegistration returns contract ID using its name
func (s *SubstrateImpl) GetContractIDByNameRegistration(name string) (uint64, error) {
	res, err := s.Substrate.GetContractIDByNameRegistration(name)
	return res, normalizeNotFoundErrors(err)
}

// CreateNodeContract creates a new node contract
func (s *SubstrateImpl) CreateNodeContract(identity substrate.Identity, node uint32, body string, hash string, publicIPs uint32, solutionProviderID *uint64) (uint64, error) {
	res, err := s.Substrate.CreateNodeContract(identity, node, body, hash, publicIPs, solutionProviderID)
	return res, normalizeNotFoundErrors(err)
}

// UpdateNodeContract updates a new name contract
func (s *SubstrateImpl) UpdateNodeContract(identity substrate.Identity, contract uint64, body string, hash string) (uint64, error) {
	res, err := s.Substrate.UpdateNodeContract(identity, contract, body, hash)
	return res, normalizeNotFoundErrors(err)
}

// GetContract returns a contract given its ID
func (s *SubstrateImpl) GetContract(contractID uint64) (Contract, error) {
	contract, err := s.Substrate.GetContract(contractID)
	return Contract{contract}, normalizeNotFoundErrors(err)
}

// CancelContract cancels a contract
func (s *SubstrateImpl) CancelContract(identity substrate.Identity, contractID uint64) error {
	if contractID == 0 {
		return nil
	}
	if err := s.Substrate.CancelContract(identity, contractID); err != nil && err.Error() != "ContractNotExists" {
		return normalizeNotFoundErrors(err)
	}
	return nil
}

// EnsureContractCanceled ensures a canceled contract
func (s *SubstrateImpl) EnsureContractCanceled(identity substrate.Identity, contractID uint64) error {
	if contractID == 0 {
		return nil
	}
	if err := s.Substrate.CancelContract(identity, contractID); err != nil && err.Error() != "ContractNotExists" {
		return normalizeNotFoundErrors(err)
	}
	return nil
}

// DeleteInvalidContracts deletes invalid contracts
func (s *SubstrateImpl) DeleteInvalidContracts(contracts map[uint32]uint64) error {
	for node, contractID := range contracts {
		valid, err := s.IsValidContract(contractID)
		// TODO: handle pause
		if err != nil {
			return normalizeNotFoundErrors(err)
		}
		if !valid {
			delete(contracts, node)
		}
	}
	return nil
}

// IsValidContract checks if a contract is invalid
func (s *SubstrateImpl) IsValidContract(contractID uint64) (bool, error) {
	if contractID == 0 {
		return false, nil
	}
	contract, err := s.Substrate.GetContract(contractID)
	err = normalizeNotFoundErrors(err)
	// TODO: handle pause
	if errors.Is(err, substrate.ErrNotFound) || (contract != nil && !contract.State.IsCreated) {
		return false, nil
	} else if err != nil {
		return true, errors.Wrapf(err, "couldn't get contract %d info", contractID)
	}
	return true, nil
}

// InvalidateNameContract invalidate a name contract
func (s *SubstrateImpl) InvalidateNameContract(
	ctx context.Context,
	identity substrate.Identity,
	contractID uint64,
	name string,
) (uint64, error) {
	if contractID == 0 {
		return 0, nil
	}
	contract, err := s.Substrate.GetContract(contractID)
	err = normalizeNotFoundErrors(err)
	if errors.Is(err, substrate.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.Wrap(err, "couldn't get name contract info")
	}
	// TODO: paused?
	if !contract.State.IsCreated {
		return 0, nil
	}
	if contract.ContractType.NameContract.Name != name {
		err := s.Substrate.CancelContract(identity, contractID)
		if err != nil {
			return 0, errors.Wrap(normalizeNotFoundErrors(err), "failed to cleanup unmatched name contract")
		}
		return 0, nil
	}

	return contractID, nil
}

// Close closes substrate
func (s *SubstrateImpl) Close() {
	s.Substrate.Close()
}

func normalizeNotFoundErrors(err error) error {
	if errors.Is(err, substrate.ErrNotFound) {
		return substrate.ErrNotFound
	}

	if errors.Is(err, substrate.ErrAccountNotFound) {
		return substrate.ErrAccountNotFound
	}
	return err
}
