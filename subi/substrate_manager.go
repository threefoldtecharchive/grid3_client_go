package subi

import (
	"context"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
	client "github.com/threefoldtech/substrate-client"
)

type ManagerInterface interface {
	client.Manager
	SubstrateExt() (SubstrateExt, error)
}

type Manager struct {
	client.Manager
	// versioned subdev.Manager
}

func NewManager(url ...string) ManagerInterface {
	return &Manager{client.NewManager(url...)}
}

func (m *Manager) SubstrateExt() (SubstrateExt, error) {
	sub, err := m.Manager.Substrate()
	return &SubstrateImpl{sub}, err
}

type Substrate interface {
	CancelContract(identity Identity, contractID uint64) error
	CreateNodeContract(identity Identity, node uint32, body string, hash string, publicIPs uint32, solutionProviderID uint64) (uint64, error)
	UpdateNodeContract(identity Identity, contract uint64, body string, hash string) (uint64, error)
	Close()
	GetTwinByPubKey(pk []byte) (uint32, error)
}
type SubstrateExt interface {
	Substrate
	EnsureContractCanceled(identity Identity, contractID uint64) error
	DeleteInvalidContracts(contracts map[uint32]uint64) error
	IsValidContract(contractID uint64) (bool, error)
	InvalidateNameContract(
		ctx context.Context,
		identity Identity,
		contractID uint64,
		name string,
	) (uint64, error)
	GetContract(id uint64) (Contract, error)
	GetNodeTwin(id uint32) (uint32, error)
	CreateNameContract(identity Identity, name string) (uint64, error)
	GetAccount(identity Identity) (types.AccountInfo, error)
	GetTwinIP(twinID uint32) (string, error)
	GetContractIDByNameRegistration(name string) (uint64, error)
}
type SubstrateImpl struct {
	*client.Substrate
}

func (s *SubstrateImpl) GetContractIDByNameRegistration(name string) (uint64, error) {
	res, err := s.Substrate.GetContractIDByNameRegistration(name)
	return res, terr(err)
}
func (s *SubstrateImpl) GetTwinIP(id uint32) (string, error) {
	twin, err := s.Substrate.GetTwin(id)
	if err != nil {
		return "", terr(err)
	}
	return twin.IP, nil
}
func (s *SubstrateImpl) GetAccount(identity Identity) (types.AccountInfo, error) {
	res, err := s.Substrate.GetAccount(identity)
	return res, terr(err)
}
func (s *SubstrateImpl) CreateNameContract(identity Identity, name string) (uint64, error) {
	return s.Substrate.CreateNameContract(identity, name)
}
func (s *SubstrateImpl) GetNodeTwin(id uint32) (uint32, error) {
	node, err := s.Substrate.GetNode(id)
	if err != nil {
		return 0, terr(err)
	}
	return uint32(node.TwinID), nil
}
func (s *SubstrateImpl) UpdateNodeContract(identity Identity, contract uint64, body string, hash string) (uint64, error) {
	res, err := s.Substrate.UpdateNodeContract(identity, contract, body, hash)
	return res, terr(err)
}
func (s *SubstrateImpl) CreateNodeContract(identity Identity, node uint32, body string, hash string, publicIPs uint32, solutionProviderID uint64) (uint64, error) {
	res, err := s.Substrate.CreateNodeContract(identity, node, body, hash, publicIPs, &solutionProviderID)
	return res, terr(err)
}
func (s *SubstrateImpl) GetContract(contractID uint64) (Contract, error) {
	contract, err := s.Substrate.GetContract(contractID)
	return &ContractImpl{contract}, terr(err)
}
func (s *SubstrateImpl) CancelContract(identity Identity, contractID uint64) error {
	if contractID == 0 {
		return nil
	}
	if err := s.Substrate.CancelContract(identity, contractID); err != nil && err.Error() != "ContractNotExists" {
		return terr(err)
	}
	return nil
}
func (s *SubstrateImpl) EnsureContractCanceled(identity Identity, contractID uint64) error {
	if contractID == 0 {
		return nil
	}
	if err := s.Substrate.CancelContract(identity, contractID); err != nil && err.Error() != "ContractNotExists" {
		return terr(err)
	}
	return nil
}

func (s *SubstrateImpl) DeleteInvalidContracts(contracts map[uint32]uint64) error {
	for node, contractID := range contracts {
		valid, err := s.IsValidContract(contractID)
		// TODO: handle pause
		if err != nil {
			return terr(err)
		}
		if !valid {
			delete(contracts, node)
		}
	}
	return nil
}

func (s *SubstrateImpl) IsValidContract(contractID uint64) (bool, error) {
	if contractID == 0 {
		return false, nil
	}
	contract, err := s.Substrate.GetContract(contractID)
	err = terr(err)
	// TODO: handle pause
	if errors.Is(err, ErrNotFound) || (contract != nil && !contract.State.IsCreated) {
		return false, nil
	} else if err != nil {
		return true, errors.Wrapf(err, "couldn't get contract %d info", contractID)
	}
	return true, nil
}

func (s *SubstrateImpl) InvalidateNameContract(
	ctx context.Context,
	identity Identity,
	contractID uint64,
	name string,
) (uint64, error) {
	if contractID == 0 {
		return 0, nil
	}
	contract, err := s.Substrate.GetContract(contractID)
	err = terr(err)
	if errors.Is(err, ErrNotFound) {
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
			return 0, errors.Wrap(terr(err), "failed to cleanup unmatching name contract")
		}
		return 0, nil
	}

	return contractID, nil
}
