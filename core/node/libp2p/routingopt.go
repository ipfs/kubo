package libp2p

import (
	"context"

	"github.com/ipfs/go-datastore"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
)

type RoutingOption func(
	context.Context,
	host.Host,
	datastore.Batching,
	record.Validator,
	...peer.AddrInfo,
) (routing.Routing, error)

func constructDHTRouting(mode dht.ModeOpt, overwrite ...dual.Option) func(
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
			append(
				[]dual.Option{
					dual.DHTOption(
						dht.Concurrency(10),
						dht.Mode(mode),
						dht.Datastore(dstore),
						dht.Validator(validator),
						dht.BootstrapPeers(bootstrapPeers...),
					),
				},
				overwrite...,
			)...,
		)
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
	DHTOption                   RoutingOption = constructDHTRouting(dht.ModeAuto)
	DHTAutoWanNoDiversityOption               = constructDHTRouting(dht.ModeAuto,
		dual.WanDHTOption(dht.RoutingTablePeerDiversityFilter(nil)),
	)
	DHTClientOption               = constructDHTRouting(dht.ModeClient)
	DHTServerOption               = constructDHTRouting(dht.ModeServer)
	DHTServerWanNoDiversityOption = constructDHTRouting(dht.ModeServer,
		dual.WanDHTOption(dht.RoutingTablePeerDiversityFilter(nil)),
	)
	NilRouterOption = constructNilRouting
)
