package coreapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/namesys"

	ipath "github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-crypto"
	ci "github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-peer"
)

type NameAPI CoreAPI

type ipnsEntry struct {
	name  string
	value coreiface.Path
}

// Name returns the ipnsEntry name.
func (e *ipnsEntry) Name() string {
	return e.name
}

// Value returns the ipnsEntry value.
func (e *ipnsEntry) Value() coreiface.Path {
	return e.value
}

// Publish announces new IPNS name and returns the new IPNS entry.
func (api *NameAPI) Publish(ctx context.Context, p coreiface.Path, opts ...caopts.NamePublishOption) (coreiface.IpnsEntry, error) {
	if err := api.checkPublishAllowed(); err != nil {
		return nil, err
	}

	options, err := caopts.NamePublishOptions(opts...)
	if err != nil {
		return nil, err
	}

	err = api.checkOnline(options.AllowOffline)
	if err != nil {
		return nil, err
	}

	pth, err := ipath.ParsePath(p.String())
	if err != nil {
		return nil, err
	}

	k, err := keylookup(api.privateKey, api.repo.Keystore(), options.Key)
	if err != nil {
		return nil, err
	}

	if options.TTL != nil {
		ctx = context.WithValue(ctx, "ipns-publish-ttl", *options.TTL)
	}

	eol := time.Now().Add(options.ValidTime)
	err = api.namesys.PublishWithEOL(ctx, k, pth, eol)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	return &ipnsEntry{
		name:  pid.Pretty(),
		value: p,
	}, nil
}

func (api *NameAPI) Search(ctx context.Context, name string, opts ...caopts.NameResolveOption) (<-chan coreiface.IpnsResult, error) {
	options, err := caopts.NameResolveOptions(opts...)
	if err != nil {
		return nil, err
	}

	err = api.checkOnline(true)
	if err != nil {
		return nil, err
	}

	var resolver namesys.Resolver = api.namesys

	if !options.Cache {
		resolver = namesys.NewNameSystem(api.routing, api.repo.Datastore(), 0)
	}

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	out := make(chan coreiface.IpnsResult)
	go func() {
		defer close(out)
		for res := range resolver.ResolveAsync(ctx, name, options.ResolveOpts...) {
			p, _ := coreiface.ParsePath(res.Path.String())

			select {
			case out <- coreiface.IpnsResult{Path: p, Err: res.Err}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Resolve attempts to resolve the newest version of the specified name and
// returns its path.
func (api *NameAPI) Resolve(ctx context.Context, name string, opts ...caopts.NameResolveOption) (coreiface.Path, error) {
	results, err := api.Search(ctx, name, opts...)
	if err != nil {
		return nil, err
	}

	err = coreiface.ErrResolveFailed
	var p coreiface.Path

	for res := range results {
		p, err = res.Path, res.Err
		if err != nil {
			break
		}
	}

	return p, err
}

func keylookup(self ci.PrivKey, kstore keystore.Keystore, k string) (crypto.PrivKey, error) {
	if k == "self" {
		return self, nil
	}

	res, err := kstore.Get(k)
	if res != nil {
		return res, nil
	}

	if err != nil && err != keystore.ErrNoSuchKey {
		return nil, err
	}

	keys, err := kstore.List()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		privKey, err := kstore.Get(key)
		if err != nil {
			return nil, err
		}

		pubKey := privKey.GetPublic()

		pid, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return nil, err
		}

		if pid.Pretty() == k {
			return privKey, nil
		}
	}

	return nil, fmt.Errorf("no key by the given name or PeerID was found")
}
