package libp2p

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

type RoutingOption func(
	context.Context,
	host.Host,
	datastore.Batching,
	record.Validator,
	...peer.AddrInfo,
) (routing.Routing, error)

// Default HTTP routers used in parallel to DHT when Routing.Type = "auto"
var defaultHTTPRouters = []string{
	"https://cid.contact", // https://github.com/ipfs/kubo/issues/9422#issuecomment-1338142084
	// TODO: add an independent router from Cloudflare
}

// ConstructDefaultRouting returns routers used when Routing.Type is unset or set to "auto"
func ConstructDefaultRouting(peerID string, addrs []string, privKey string) func(
	ctx context.Context,
	host host.Host,
	dstore datastore.Batching,
	validator record.Validator,
	bootstrapPeers ...peer.AddrInfo,
) (routing.Routing, error) {
	return func(
		ctx context.Context,
		host host.Host,
		dstore datastore.Batching,
		validator record.Validator,
		bootstrapPeers ...peer.AddrInfo,
	) (routing.Routing, error) {
		// Defined routers will be queried in parallel (optimizing for response speed)
		// Different trade-offs can be made by setting Routing.Type = "custom" with own Routing.Routers
		var routers []*routinghelpers.ParallelRouter

		// Run the default DHT routing (same as Routing.Type = "dht")
		dhtRouting, err := DHTOption(ctx, host, dstore, validator, bootstrapPeers...)
		if err != nil {
			return nil, err
		}
		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:       dhtRouting,
			IgnoreError:  false,
			Timeout:      5 * time.Minute, // https://github.com/ipfs/kubo/pull/9475#discussion_r1042501333
			ExecuteAfter: 0,
		})

		// Append HTTP routers for additional speed
		for _, endpoint := range defaultHTTPRouters {
			httpRouter, err := irouting.ConstructHTTPRouter(endpoint, peerID, addrs, privKey)
			if err != nil {
				return nil, err
			}
			routers = append(routers, &routinghelpers.ParallelRouter{
				Router:       httpRouter,
				IgnoreError:  true,             // https://github.com/ipfs/kubo/pull/9475#discussion_r1042507387
				Timeout:      15 * time.Second, // 5x server value from https://github.com/ipfs/kubo/pull/9475#discussion_r1042428529
				ExecuteAfter: 0,
			})
		}

		routing := routinghelpers.NewComposableParallel(routers)
		return routing, nil
	}
}

// constructDHTRouting is used when Routing.Type = "dht"
func constructDHTRouting(mode dht.ModeOpt) func(
	ctx context.Context,
	host host.Host,
	dstore datastore.Batching,
	validator record.Validator,
	bootstrapPeers ...peer.AddrInfo,
) (routing.Routing, error) {
	return func(
		ctx context.Context,
		host host.Host,
		dstore datastore.Batching,
		validator record.Validator,
		bootstrapPeers ...peer.AddrInfo,
	) (routing.Routing, error) {
		return dual.New(
			ctx, host,
			dual.DHTOption(
				dht.Concurrency(10),
				dht.Mode(mode),
				dht.Datastore(dstore),
				dht.Validator(validator)),
			dual.WanDHTOption(dht.BootstrapPeers(bootstrapPeers...)),
		)
	}
}

// ConstructDelegatedRouting is used when Routing.Type = "custom"
func ConstructDelegatedRouting(routers config.Routers, methods config.Methods, peerID string, addrs []string, privKey string) func(
	ctx context.Context,
	host host.Host,
	dstore datastore.Batching,
	validator record.Validator,
	bootstrapPeers ...peer.AddrInfo,
) (routing.Routing, error) {
	return func(
		ctx context.Context,
		host host.Host,
		dstore datastore.Batching,
		validator record.Validator,
		bootstrapPeers ...peer.AddrInfo,
	) (routing.Routing, error) {
		return irouting.Parse(routers, methods,
			&irouting.ExtraDHTParams{
				BootstrapPeers: bootstrapPeers,
				Host:           host,
				Validator:      validator,
				Datastore:      dstore,
				Context:        ctx,
			},
			&irouting.ExtraHTTPParams{
				PeerID:     peerID,
				Addrs:      addrs,
				PrivKeyB64: privKey,
			})
	}
}

func constructNilRouting(
	ctx context.Context,
	host host.Host,
	dstore datastore.Batching,
	validator record.Validator,
	bootstrapPeers ...peer.AddrInfo,
) (routing.Routing, error) {
	return routinghelpers.Null{}, nil
}

var (
	DHTOption       RoutingOption = constructDHTRouting(dht.ModeAuto)
	DHTClientOption               = constructDHTRouting(dht.ModeClient)
	DHTServerOption               = constructDHTRouting(dht.ModeServer)
	NilRouterOption               = constructNilRouting
)
