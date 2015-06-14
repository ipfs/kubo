package keystore

import (
	"errors"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
)

type memKeystore struct {
	ks map[string]ci.PrivKey
}

func NewInMemKeystore() Keystore {
	return &memKeystore{make(map[string]ci.PrivKey)}
}

func (ks *memKeystore) GetKey(name string) (ci.PrivKey, error) {
	k, found := ks.ks[name]
	if found {
		return k, nil
	}

	return nil, errors.New("key not found")
}

func (ks *memKeystore) PutKey(name string, k ci.PrivKey) error {
	ks.ks[name] = k
	return nil
}
