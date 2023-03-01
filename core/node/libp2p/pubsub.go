package libp2p

import (
	"context"
	"errors"
	"fmt"

	datastore "github.com/ipfs/go-datastore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
)

type P2PPubSubIn struct {
	fx.In

	Repo      repo.Repo
	Host      host.Host
	Discovery discovery.Discovery
}

func FloodSub(pubsubOptions ...pubsub.Option) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, params P2PPubSubIn) (service *pubsub.PubSub, err error) {
		return pubsub.NewFloodSub(
			helpers.LifecycleCtx(mctx, lc),
			params.Host,
			append(pubsubOptions,
				pubsub.WithDiscovery(params.Discovery),
				pubsub.WithDefaultValidator(pubsub.NewBasicSeqnoValidator(makePubSubMetadataStore(params.Repo.Datastore()))))...,
		)
	}
}

func GossipSub(pubsubOptions ...pubsub.Option) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, params P2PPubSubIn) (service *pubsub.PubSub, err error) {
		return pubsub.NewGossipSub(
			helpers.LifecycleCtx(mctx, lc),
			params.Host,
			append(
				pubsubOptions,
				pubsub.WithDiscovery(params.Discovery),
				pubsub.WithFloodPublish(true),
				pubsub.WithDefaultValidator(pubsub.NewBasicSeqnoValidator(makePubSubMetadataStore(params.Repo.Datastore()))))...,
		)
	}
}

func makePubSubMetadataStore(ds datastore.Datastore) pubsub.PeerMetadataStore {
	return &pubsubMetadataStore{ds: ds}
}

type pubsubMetadataStore struct {
	ds datastore.Datastore
}

func (m *pubsubMetadataStore) Get(ctx context.Context, p peer.ID) ([]byte, error) {
	k := datastore.NewKey(fmt.Sprintf("/pubsub/seqno/%s", p))

	v, err := m.ds.Get(ctx, k)
	if err != nil && errors.Is(err, datastore.ErrNotFound) {
		return nil, nil
	}

	return v, err
}

func (m *pubsubMetadataStore) Put(ctx context.Context, p peer.ID, v []byte) error {
	k := datastore.NewKey(fmt.Sprintf("/pubsub/seqno/%s", p))
	return m.ds.Put(ctx, k, v)
}
