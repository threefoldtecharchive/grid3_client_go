package subi

import (
	"github.com/pkg/errors"
	client "github.com/threefoldtech/substrate-client"
)

var ErrNotFound = client.ErrNotFound
var ErrAccountNotFound = client.ErrAccountNotFound

func terr(err error) error {
	if errors.Is(err, client.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, client.ErrAccountNotFound) {
		return ErrAccountNotFound
	}
	return err
}
