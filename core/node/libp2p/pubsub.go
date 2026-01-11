package libp2p

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
)

type pubsubParams struct {
	fx.In

	Repo      repo.Repo
	Host      host.Host
	Discovery discovery.Discovery
}

func FloodSub(pubsubOptions ...pubsub.Option) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, params pubsubParams) (service *pubsub.PubSub, err error) {
		return pubsub.NewFloodSub(
			helpers.LifecycleCtx(mctx, lc),
			params.Host,
			append(pubsubOptions,
				pubsub.WithDiscovery(params.Discovery),
				pubsub.WithDefaultValidator(newSeqnoValidator(params.Repo.Datastore())))...,
		)
	}
}

func GossipSub(pubsubOptions ...pubsub.Option) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, params pubsubParams) (service *pubsub.PubSub, err error) {
		return pubsub.NewGossipSub(
			helpers.LifecycleCtx(mctx, lc),
			params.Host,
			append(pubsubOptions,
				pubsub.WithDiscovery(params.Discovery),
				pubsub.WithFloodPublish(true), // flood own publications to all peers for reliable IPNS delivery
				pubsub.WithDefaultValidator(newSeqnoValidator(params.Repo.Datastore())))...,
		)
	}
}

func newSeqnoValidator(ds datastore.Datastore) pubsub.ValidatorEx {
	return pubsub.NewBasicSeqnoValidator(&seqnoStore{ds: ds}, slog.New(logging.SlogHandler()).With("logger", "pubsub"))
}

// SeqnoStorePrefix is the datastore prefix for pubsub seqno validator state.
const SeqnoStorePrefix = "/pubsub/seqno/"

// seqnoStore implements pubsub.PeerMetadataStore using the repo datastore.
// It stores the maximum seen sequence number per peer to prevent message
// cycles when network diameter exceeds the timecache span.
type seqnoStore struct {
	ds datastore.Datastore
}

var _ pubsub.PeerMetadataStore = (*seqnoStore)(nil)

// Get returns the stored seqno for a peer, or (nil, nil) if the peer is unknown.
// Returning (nil, nil) for unknown peers allows BasicSeqnoValidator to accept
// the first message from any peer.
func (s *seqnoStore) Get(ctx context.Context, p peer.ID) ([]byte, error) {
	key := datastore.NewKey(SeqnoStorePrefix + p.String())
	val, err := s.ds.Get(ctx, key)
	if errors.Is(err, datastore.ErrNotFound) {
		return nil, nil
	}
	return val, err
}

// Put stores the seqno for a peer.
func (s *seqnoStore) Put(ctx context.Context, p peer.ID, val []byte) error {
	key := datastore.NewKey(SeqnoStorePrefix + p.String())
	return s.ds.Put(ctx, key, val)
}
