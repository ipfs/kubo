package coreapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	keystore "github.com/ipfs/go-ipfs/keystore"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ipath "github.com/ipfs/go-ipfs/path"

	offline "gx/ipfs/QmZRcGYvxdauCd7hHnMYLYqcZRaDjv24c7eUNyJojAcdBb/go-ipfs-routing/offline"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

type NameAPI struct {
	*CoreAPI
	*caopts.NameOptions
}

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

// Resolve attempts to resolve the newest version of the specified name and
// returns its path.
func (api *NameAPI) Resolve(ctx context.Context, name string, opts ...caopts.NameResolveOption) (coreiface.Path, error) {
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
		offroute := offline.NewOfflineRouter(n.Repo.Datastore(), n.PrivateKey)
		resolver = namesys.NewRoutingResolver(offroute, 0)
	}

	if !options.Cache {
		resolver = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), 0)
	}

	depth := 1
	if options.Recursive {
		depth = namesys.DefaultDepthLimit
	}

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	output, err := resolver.ResolveN(ctx, name, depth)
	if err != nil {
		return nil, err
	}

	return &path{path: output}, nil
}

func (api *NameAPI) core() coreiface.CoreAPI {
	return api.CoreAPI
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
