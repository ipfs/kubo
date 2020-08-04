package coreapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"

	ipfspath "github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

type KeyAPI CoreAPI

type key struct {
	name   string
	peerID peer.ID
}

// Name returns the key name
func (k *key) Name() string {
	return k.name
}

// Path returns the path of the key.
func (k *key) Path() path.Path {
	return path.New(ipfspath.Join([]string{"/ipns", coreiface.FormatKeyID(k.peerID)}))
}

// ID returns key PeerID
func (k *key) ID() peer.ID {
	return k.peerID
}

// Generate generates new key, stores it in the keystore under the specified
// name and returns a base58 encoded multihash of its public key.
func (api *KeyAPI) Generate(ctx context.Context, name string, opts ...caopts.KeyGenerateOption) (coreiface.Key, error) {
	options, err := caopts.KeyGenerateOptions(opts...)
	if err != nil {
		return nil, err
	}

	if name == "self" {
		return nil, fmt.Errorf("cannot create key with name 'self'")
	}

	_, err = api.repo.Keystore().Get(name)
	if err == nil {
		return nil, fmt.Errorf("key with name '%s' already exists", name)
	}

	var sk crypto.PrivKey
	var pk crypto.PubKey

	switch options.Algorithm {
	case "rsa":
		if options.Size == -1 {
			options.Size = caopts.DefaultRSALen
		}

		priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.RSA, options.Size, rand.Reader)
		if err != nil {
			return nil, err
		}

		sk = priv
		pk = pub
	case "ed25519":
		priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, err
		}

		sk = priv
		pk = pub
	default:
		return nil, fmt.Errorf("unrecognized key type: %s", options.Algorithm)
	}

	err = api.repo.Keystore().Put(name, sk)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, err
	}

	return &key{name, pid}, nil
}

// List returns a list keys stored in keystore.
func (api *KeyAPI) List(ctx context.Context) ([]coreiface.Key, error) {
	keys, err := api.repo.Keystore().List()
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)

	out := make([]coreiface.Key, len(keys)+1)
	out[0] = &key{"self", api.identity}

	for n, k := range keys {
		privKey, err := api.repo.Keystore().Get(k)
		if err != nil {
			return nil, err
		}

		pubKey := privKey.GetPublic()

		pid, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return nil, err
		}

		out[n+1] = &key{k, pid}
	}
	return out, nil
}

// Rename renames `oldName` to `newName`. Returns the key and whether another
// key was overwritten, or an error.
func (api *KeyAPI) Rename(ctx context.Context, oldName string, newName string, opts ...caopts.KeyRenameOption) (coreiface.Key, bool, error) {
	options, err := caopts.KeyRenameOptions(opts...)
	if err != nil {
		return nil, false, err
	}

	ks := api.repo.Keystore()

	if oldName == "self" {
		return nil, false, fmt.Errorf("cannot rename key with name 'self'")
	}

	if newName == "self" {
		return nil, false, fmt.Errorf("cannot overwrite key with name 'self'")
	}

	oldKey, err := ks.Get(oldName)
	if err != nil {
		return nil, false, fmt.Errorf("no key named %s was found", oldName)
	}

	pubKey := oldKey.GetPublic()

	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, false, err
	}

	// This is important, because future code will delete key `oldName`
	// even if it is the same as newName.
	if newName == oldName {
		return &key{oldName, pid}, false, nil
	}

	overwrite := false
	if options.Force {
		exist, err := ks.Has(newName)
		if err != nil {
			return nil, false, err
		}

		if exist {
			overwrite = true
			err := ks.Delete(newName)
			if err != nil {
				return nil, false, err
			}
		}
	}

	err = ks.Put(newName, oldKey)
	if err != nil {
		return nil, false, err
	}

	return &key{newName, pid}, overwrite, ks.Delete(oldName)
}

// Remove removes keys from keystore. Returns ipns path of the removed key.
func (api *KeyAPI) Remove(ctx context.Context, name string) (coreiface.Key, error) {
	ks := api.repo.Keystore()

	if name == "self" {
		return nil, fmt.Errorf("cannot remove key with name 'self'")
	}

	removed, err := ks.Get(name)
	if err != nil {
		return nil, fmt.Errorf("no key named %s was found", name)
	}

	pubKey := removed.GetPublic()

	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, err
	}

	err = ks.Delete(name)
	if err != nil {
		return nil, err
	}

	return &key{"", pid}, nil
}

func (api *KeyAPI) Self(ctx context.Context) (coreiface.Key, error) {
	if api.identity == "" {
		return nil, errors.New("identity not loaded")
	}

	return &key{"self", api.identity}, nil
}
