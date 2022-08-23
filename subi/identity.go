package subi

import (
	client "github.com/threefoldtech/substrate-client"
)

type Identity client.Identity

func NewIdentityFromEd25519Phrase(phrase string) (Identity, error) {
	id, err := client.NewIdentityFromEd25519Phrase(phrase)
	return Identity(id), err
}

func NewIdentityFromSr25519Phrase(phrase string) (Identity, error) {
	id, err := client.NewIdentityFromSr25519Phrase(phrase)
	return Identity(id), err
}
