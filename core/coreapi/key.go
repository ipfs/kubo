package coreapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/tracing"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type KeyAPI CoreAPI

type key struct {
	name   string
	peerID peer.ID
	path   path.Path
}

func newKey(name string, pid peer.ID) (*key, error) {
	p, err := path.NewPath("/ipns/" + ipns.NameFromPeer(pid).String())
	if err != nil {
		return nil, err
	}
	return &key{
		name:   name,
		peerID: pid,
		path:   p,
	}, nil
}

// Name returns the key name
func (k *key) Name() string {
	return k.name
}

// Path returns the path of the key.
func (k *key) Path() path.Path {
	return k.path
}

// ID returns key PeerID
func (k *key) ID() peer.ID {
	return k.peerID
}

// Generate generates new key, stores it in the keystore under the specified
// name and returns a base58 encoded multihash of its public key.
func (api *KeyAPI) Generate(ctx context.Context, name string, opts ...caopts.KeyGenerateOption) (coreiface.Key, error) {
	_, span := tracing.Span(ctx, "CoreAPI.KeyAPI", "Generate", trace.WithAttributes(attribute.String("name", name)))
	defer span.End()

	options, err := caopts.KeyGenerateOptions(opts...)
	if err != nil {
		return nil, err
	}

	if name == "self" {
		return nil, errors.New("cannot create key with name 'self'")
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

	return newKey(name, pid)
}

// List returns a list keys stored in keystore.
func (api *KeyAPI) List(ctx context.Context) ([]coreiface.Key, error) {
	_, span := tracing.Span(ctx, "CoreAPI.KeyAPI", "List")
	defer span.End()

	keys, err := api.repo.Keystore().List()
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)

	out := make([]coreiface.Key, len(keys)+1)
	out[0], err = newKey("self", api.identity)
	if err != nil {
		return nil, err
	}

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

		out[n+1], err = newKey(k, pid)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Rename renames `oldName` to `newName`. Returns the key and whether another
// key was overwritten, or an error.
func (api *KeyAPI) Rename(ctx context.Context, oldName string, newName string, opts ...caopts.KeyRenameOption) (coreiface.Key, bool, error) {
	_, span := tracing.Span(ctx, "CoreAPI.KeyAPI", "Rename", trace.WithAttributes(attribute.String("oldname", oldName), attribute.String("newname", newName)))
	defer span.End()

	options, err := caopts.KeyRenameOptions(opts...)
	if err != nil {
		return nil, false, err
	}
	span.SetAttributes(attribute.Bool("force", options.Force))

	ks := api.repo.Keystore()

	if oldName == "self" {
		return nil, false, errors.New("cannot rename key with name 'self'")
	}

	if newName == "self" {
		return nil, false, errors.New("cannot overwrite key with name 'self'")
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
		k, err := newKey(oldName, pid)
		return k, false, err
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

	err = ks.Delete(oldName)
	if err != nil {
		return nil, false, err
	}

	k, err := newKey(newName, pid)
	return k, overwrite, err
}

// Remove removes keys from keystore. Returns ipns path of the removed key.
func (api *KeyAPI) Remove(ctx context.Context, name string) (coreiface.Key, error) {
	_, span := tracing.Span(ctx, "CoreAPI.KeyAPI", "Remove", trace.WithAttributes(attribute.String("name", name)))
	defer span.End()

	ks := api.repo.Keystore()

	if name == "self" {
		return nil, errors.New("cannot remove key with name 'self'")
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

	return newKey("", pid)
}

func (api *KeyAPI) Self(ctx context.Context) (coreiface.Key, error) {
	if api.identity == "" {
		return nil, errors.New("identity not loaded")
	}

	return newKey("self", api.identity)
}

const signedMessagePrefix = "libp2p-key signed message:"

func (api *KeyAPI) Sign(ctx context.Context, name string, data []byte) (coreiface.Key, []byte, error) {
	var (
		sk  crypto.PrivKey
		err error
	)
	if name == "" || name == "self" {
		name = "self"
		sk = api.privateKey
	} else {
		sk, err = api.repo.Keystore().Get(name)
	}
	if err != nil {
		return nil, nil, err
	}

	pid, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, nil, err
	}

	key, err := newKey(name, pid)
	if err != nil {
		return nil, nil, err
	}

	data = append([]byte(signedMessagePrefix), data...)

	sig, err := sk.Sign(data)
	if err != nil {
		return nil, nil, err
	}

	return key, sig, nil
}

func (api *KeyAPI) Verify(ctx context.Context, keyOrName string, signature, data []byte) (coreiface.Key, bool, error) {
	var (
		name string
		pk   crypto.PubKey
		err  error
	)
	if keyOrName == "" || keyOrName == "self" {
		name = "self"
		pk = api.privateKey.GetPublic()
	} else if sk, err := api.repo.Keystore().Get(keyOrName); err == nil {
		name = keyOrName
		pk = sk.GetPublic()
	} else if ipnsName, err := ipns.NameFromString(keyOrName); err == nil {
		// This works for both IPNS names and Peer IDs.
		name = ""
		pk, err = ipnsName.Peer().ExtractPublicKey()
		if err != nil {
			return nil, false, err
		}
	} else {
		return nil, false, fmt.Errorf("'%q' is not a known key, an IPNS Name, or a valid PeerID", keyOrName)
	}

	pid, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, false, err
	}

	key, err := newKey(name, pid)
	if err != nil {
		return nil, false, err
	}

	data = append([]byte(signedMessagePrefix), data...)

	valid, err := pk.Verify(data, signature)
	if err != nil {
		return nil, false, err
	}

	return key, valid, nil
}
