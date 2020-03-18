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

var baseDhtOptions = []dht.Option{
	dht.Concurrency(10),
	// TODO: Enable this once we have the dual-DHTs. At the moment, this
	// will break DHTs on private IP addresses (VPNs, etc.).
	//dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
	//dht.QueryFilter(dht.PublicQueryFilter),
}

type RoutingOption func(context.Context, host.Host, datastore.Batching, record.Validator) (routing.Routing, error)

func constructDHTRouting(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.Routing, error) {
	return dht.New(
		ctx, host,
		append([]dht.Option{
			// TODO: switch to "auto" when we add dual-dht support.
			// Unfortunately, we can't set this to auto yet or
			// nobody will become a server when everyone is running
			// on a private network.
			dht.Mode(dht.ModeServer),
			dht.Datastore(dstore),
			dht.Validator(validator),
		}, baseDhtOptions...)...,
	)
}

func constructClientDHTRouting(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.Routing, error) {
	return dht.New(
		ctx, host,
		append([]dht.Option{
			dht.Mode(dht.ModeClient),
			dht.Datastore(dstore),
			dht.Validator(validator),
		}, baseDhtOptions...)...,
	)
}

var DHTOption RoutingOption = constructDHTRouting
var DHTClientOption RoutingOption = constructClientDHTRouting
var NilRouterOption RoutingOption = nilrouting.ConstructNilRouting
