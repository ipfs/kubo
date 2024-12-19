/*
**NOTE: this package is experimental.**

Package coreapi provides direct access to the core commands in IPFS. If you are
embedding IPFS directly in your Go program, this package is the public
interface you should use to read and write files or otherwise control IPFS.

If you are running IPFS as a separate process, you should use `client/rpc` to
work with it via HTTP.
*/
package coreapi

import (
	"context"
	"errors"
	"fmt"

	bserv "github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	offlinexch "github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/fetcher"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	pathresolver "github.com/ipfs/boxo/path/resolver"
	pin "github.com/ipfs/boxo/pinning/pinner"
	provider "github.com/ipfs/boxo/provider"
	offlineroute "github.com/ipfs/boxo/routing/offline"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/config"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	record "github.com/libp2p/go-libp2p-record"
	ci "github.com/libp2p/go-libp2p/core/crypto"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
	routing "github.com/libp2p/go-libp2p/core/routing"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/repo"
)

type CoreAPI struct {
	nctx context.Context

	identity   peer.ID
	privateKey ci.PrivKey

	repo       repo.Repo
	blockstore blockstore.GCBlockstore
	baseBlocks blockstore.Blockstore
	pinning    pin.Pinner

	blocks               bserv.BlockService
	dag                  ipld.DAGService
	ipldFetcherFactory   fetcher.Factory
	unixFSFetcherFactory fetcher.Factory
	peerstore            pstore.Peerstore
	peerHost             p2phost.Host
	recordValidator      record.Validator
	exchange             exchange.Interface

	namesys            namesys.NameSystem
	routing            routing.Routing
	dnsResolver        *madns.Resolver
	ipldPathResolver   pathresolver.Resolver
	unixFSPathResolver pathresolver.Resolver

	provider provider.System

	pubSub *pubsub.PubSub

	checkPublishAllowed func() error
	checkOnline         func(allowOffline bool) error

	// ONLY for re-applying options in WithOptions, DO NOT USE ANYWHERE ELSE
	nd         *core.IpfsNode
	parentOpts options.ApiSettings
}

// NewCoreAPI creates new instance of IPFS CoreAPI backed by go-ipfs Node.
func NewCoreAPI(n *core.IpfsNode, opts ...options.ApiOption) (coreiface.CoreAPI, error) {
	parentOpts, err := options.ApiOptions()
	if err != nil {
		return nil, err
	}

	return (&CoreAPI{nd: n, parentOpts: *parentOpts}).WithOptions(opts...)
}

// Unixfs returns the UnixfsAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Unixfs() coreiface.UnixfsAPI {
	return (*UnixfsAPI)(api)
}

// Block returns the BlockAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Block() coreiface.BlockAPI {
	return (*BlockAPI)(api)
}

// Dag returns the DagAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Dag() coreiface.APIDagService {
	return &dagAPI{
		api.dag,
		api,
	}
}

// Name returns the NameAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Name() coreiface.NameAPI {
	return (*NameAPI)(api)
}

// Key returns the KeyAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Key() coreiface.KeyAPI {
	return (*KeyAPI)(api)
}

// Object returns the ObjectAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Object() coreiface.ObjectAPI {
	return (*ObjectAPI)(api)
}

// Pin returns the PinAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Pin() coreiface.PinAPI {
	return (*PinAPI)(api)
}

// Swarm returns the SwarmAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Swarm() coreiface.SwarmAPI {
	return (*SwarmAPI)(api)
}

// PubSub returns the PubSubAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) PubSub() coreiface.PubSubAPI {
	return (*PubSubAPI)(api)
}

// Routing returns the RoutingAPI interface implementation backed by the kubo node
func (api *CoreAPI) Routing() coreiface.RoutingAPI {
	return (*RoutingAPI)(api)
}

// WithOptions returns api with global options applied
func (api *CoreAPI) WithOptions(opts ...options.ApiOption) (coreiface.CoreAPI, error) {
	settings := api.parentOpts // make sure to copy
	_, err := options.ApiOptionsTo(&settings, opts...)
	if err != nil {
		return nil, err
	}

	if api.nd == nil {
		return nil, errors.New("cannot apply options to api without node")
	}

	n := api.nd

	subAPI := &CoreAPI{
		nctx: n.Context(),

		identity:   n.Identity,
		privateKey: n.PrivateKey,

		repo:       n.Repo,
		blockstore: n.Blockstore,
		baseBlocks: n.BaseBlocks,
		pinning:    n.Pinning,

		blocks:               n.Blocks,
		dag:                  n.DAG,
		ipldFetcherFactory:   n.IPLDFetcherFactory,
		unixFSFetcherFactory: n.UnixFSFetcherFactory,

		peerstore:          n.Peerstore,
		peerHost:           n.PeerHost,
		namesys:            n.Namesys,
		recordValidator:    n.RecordValidator,
		exchange:           n.Exchange,
		routing:            n.Routing,
		dnsResolver:        n.DNSResolver,
		ipldPathResolver:   n.IPLDPathResolver,
		unixFSPathResolver: n.UnixFSPathResolver,

		provider: n.Provider,

		pubSub: n.PubSub,

		nd:         n,
		parentOpts: settings,
	}

	subAPI.checkOnline = func(allowOffline bool) error {
		if !n.IsOnline && !allowOffline {
			return coreiface.ErrOffline
		}
		return nil
	}

	subAPI.checkPublishAllowed = func() error {
		if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
			return errors.New("cannot manually publish while IPNS is mounted")
		}
		return nil
	}

	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	if settings.Offline {
		cs := cfg.Ipns.ResolveCacheSize
		if cs == 0 {
			cs = node.DefaultIpnsCacheSize
		}
		if cs < 0 {
			return nil, errors.New("cannot specify negative resolve cache size")
		}

		nsOptions := []namesys.Option{
			namesys.WithDatastore(subAPI.repo.Datastore()),
			namesys.WithDNSResolver(subAPI.dnsResolver),
			namesys.WithCache(cs),
			namesys.WithMaxCacheTTL(cfg.Ipns.MaxCacheTTL.WithDefault(config.DefaultIpnsMaxCacheTTL)),
		}

		subAPI.routing = offlineroute.NewOfflineRouter(subAPI.repo.Datastore(), subAPI.recordValidator)

		subAPI.namesys, err = namesys.NewNameSystem(subAPI.routing, nsOptions...)
		if err != nil {
			return nil, fmt.Errorf("error constructing namesys: %w", err)
		}

		subAPI.provider = provider.NewNoopProvider()

		subAPI.peerstore = nil
		subAPI.peerHost = nil
		subAPI.recordValidator = nil
	}

	if settings.Offline || !settings.FetchBlocks {
		subAPI.exchange = offlinexch.Exchange(subAPI.blockstore)
		subAPI.blocks = bserv.New(subAPI.blockstore, subAPI.exchange,
			bserv.WriteThrough(cfg.Datastore.WriteThrough.WithDefault(config.DefaultWriteThrough)),
		)
		subAPI.dag = dag.NewDAGService(subAPI.blocks)
	}

	return subAPI, nil
}

// getSession returns new api backed by the same node with a read-only session DAG
func (api *CoreAPI) getSession(ctx context.Context) *CoreAPI {
	sesAPI := *api

	// TODO: We could also apply this to api.blocks, and compose into writable api,
	// but this requires some changes in blockservice/merkledag
	sesAPI.dag = dag.NewReadOnlyDagService(dag.NewSession(ctx, api.dag))

	return &sesAPI
}
