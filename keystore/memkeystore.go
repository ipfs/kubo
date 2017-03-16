package keystore

import ci "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"

type MemKeystore struct {
	keys map[string]ci.PrivKey
}

func NewMemKeystore() *MemKeystore {
	return &MemKeystore{make(map[string]ci.PrivKey)}
}

func (mk *MemKeystore) Put(name string, k ci.PrivKey) error {
	if err := validateName(name); err != nil {
		return err
	}

	_, ok := mk.keys[name]
	if ok {
		return ErrKeyExists
	}

	mk.keys[name] = k
	return nil
}

func (mk *MemKeystore) Get(name string) (ci.PrivKey, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	k, ok := mk.keys[name]
	if !ok {
		return nil, ErrNoSuchKey
	}

	return k, nil
}

func (mk *MemKeystore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	delete(mk.keys, name)
	return nil
}

func (mk *MemKeystore) List() ([]string, error) {
	out := make([]string, 0, len(mk.keys))
	for k, _ := range mk.keys {
		out = append(out, k)
	}
	return out, nil
}
