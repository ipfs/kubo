package coreapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/namesys"

	"gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	ipath "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	"gx/ipfs/QmdxhyAwBrnmJFsYPK6tyHh4Yy3gK8gbULErX1dRnpUMqu/go-ipfs-routing/offline"
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
	options, err := caopts.NamePublishOptions(opts...)
	if err != nil {
		return nil, err
	}
	n := api.node

	if !n.OnlineMode() {
		if !options.AllowOffline {
			return nil, coreiface.ErrOffline
		}
		err := n.SetupOfflineRouting()
		if err != nil {
			return nil, err
		}
	}

	if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
		return nil, errors.New("cannot manually publish while IPNS is mounted")
	}

	pth, err := ipath.ParsePath(p.String())
	if err != nil {
		return nil, err
	}

	k, err := keylookup(n, options.Key)
	if err != nil {
		return nil, err
	}

	if options.TTL != nil {
		ctx = context.WithValue(ctx, "ipns-publish-ttl", *options.TTL)
	}

	eol := time.Now().Add(options.ValidTime)
	err = n.Namesys.PublishWithEOL(ctx, k, pth, eol)
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

	n := api.node

	if !n.OnlineMode() {
		err := n.SetupOfflineRouting()
		if err != nil {
			return nil, err
		}
	}

	var resolver namesys.Resolver = n.Namesys

	if options.Local && !options.Cache {
		return nil, errors.New("cannot specify both local and nocache")
	}

	if options.Local {
		offroute := offline.NewOfflineRouter(n.Repo.Datastore(), n.RecordValidator)
		resolver = namesys.NewIpnsResolver(offroute)
	}

	if !options.Cache {
		resolver = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), 0)
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

func keylookup(n *core.IpfsNode, k string) (crypto.PrivKey, error) {
	res, err := n.GetKey(k)
	if res != nil {
		return res, nil
	}

	if err != nil && err != keystore.ErrNoSuchKey {
		return nil, err
	}

	keys, err := n.Repo.Keystore().List()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		privKey, err := n.Repo.Keystore().Get(key)
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
