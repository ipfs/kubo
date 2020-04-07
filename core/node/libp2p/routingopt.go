package libp2p

import (
	"context"

	"github.com/ipfs/go-datastore"
	nilrouting "github.com/ipfs/go-ipfs-routing/none"
	host "github.com/libp2p/go-libp2p-core/host"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
)

type RoutingOption func(context.Context, host.Host, datastore.Batching, record.Validator) (routing.Routing, error)

func constructDHTRouting(mode dht.ModeOpt) func(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.Routing, error) {
	return func(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.Routing, error) {
		return dht.New(
			ctx, host,
			dht.Concurrency(10),
			dht.Mode(mode),
			dht.Datastore(dstore),
			dht.Validator(validator),
		)
	}
}

var (
	// FIXME: Set this to dht.ModeAuto once we resolve
	// https://github.com/libp2p/go-libp2p-kad-dht/issues/564
	DHTOption       RoutingOption = constructDHTRouting(dht.ModeServer)
	DHTClientOption               = constructDHTRouting(dht.ModeClient)
	DHTServerOption               = constructDHTRouting(dht.ModeServer)
	NilRouterOption               = nilrouting.ConstructNilRouting
)
