package coreapi

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

type KeyAPI struct {
	*CoreAPI
	*caopts.KeyOptions
}

func (api *KeyAPI) Generate(ctx context.Context, name string, opts ...caopts.KeyGenerateOption) (string, error) {
	options, err := caopts.KeyGenerateOptions(opts...)
	if err != nil {
		return "", err
	}

	var sk crypto.PrivKey
	var pk crypto.PubKey

	switch options.Algorithm {
	case "rsa":
		if options.Size == 0 {
			return "", fmt.Errorf("please specify a key size with --size")
		}

		priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.RSA, options.Size, rand.Reader)
		if err != nil {
			return "", err
		}

		sk = priv
		pk = pub
	case "ed25519":
		priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return "", err
		}

		sk = priv
		pk = pub
	default:
		return "", fmt.Errorf("unrecognized key type: %s", options.Algorithm)
	}

	err = api.node.Repo.Keystore().Put(name, sk)
	if err != nil {
		return "", err
	}

	pid, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}

	return pid.String(), nil
}

func (api *KeyAPI) List(ctx context.Context) (map[string]string, error) {
	keys, err := api.node.Repo.Keystore().List()
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)

	out := make(map[string]string, len(keys)+1)
	out["self"] = api.node.Identity.Pretty()

	for _, key := range keys {
		privKey, err := api.node.Repo.Keystore().Get(key)
		if err != nil {
			return nil, err
		}

		pubKey := privKey.GetPublic()

		pid, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return nil, err
		}

		out[key] = pid.Pretty()
	}
	return out, nil
}

func (api *KeyAPI) Rename(ctx context.Context, oldName string, newName string, opts ...caopts.KeyRenameOption) (string, bool, error) {
	options, err := caopts.KeyRenameOptions(opts...)
	if newName == "self" {
		return "", false, err
	}

	ks := api.node.Repo.Keystore()

	if oldName == "self" {
		return "", false, fmt.Errorf("cannot rename key with name 'self'")
	}

	if newName == "self" {
		return "", false, fmt.Errorf("cannot overwrite key with name 'self'")
	}

	oldKey, err := ks.Get(oldName)
	if err != nil {
		return "", false, fmt.Errorf("no key named %s was found", oldName)
	}

	pubKey := oldKey.GetPublic()

	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", false, err
	}

	overwrite := false
	if options.Force {
		exist, err := ks.Has(newName)
		if err != nil {
			return "", false, err
		}

		if exist {
			overwrite = true
			err := ks.Delete(newName)
			if err != nil {
				return "", false, err
			}
		}
	}

	err = ks.Put(newName, oldKey)
	if err != nil {
		return "", false, err
	}

	return pid.Pretty(), overwrite, ks.Delete(oldName)
}

func (api *KeyAPI) Remove(ctx context.Context, name string) (string, error) {
	ks := api.node.Repo.Keystore()

	if name == "self" {
		return "", fmt.Errorf("cannot remove key with name 'self'")
	}

	removed, err := ks.Get(name)
	if err != nil {
		return "", fmt.Errorf("no key named %s was found", name)
	}

	pubKey := removed.GetPublic()

	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", err
	}

	err = ks.Delete(name)
	if err != nil {
		return "", err
	}

	return pid.Pretty(), nil
}

func (api *KeyAPI) core() coreiface.CoreAPI {
	return api.CoreAPI
}
