package coreapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cid "github.com/ipfs/go-cid"
	floodsub "github.com/libp2p/go-floodsub"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

type PubSubAPI CoreAPI

type pubSubSubscription struct {
	cancel       context.CancelFunc
	subscription *floodsub.Subscription
}

type pubSubMessage struct {
	msg *floodsub.Message
}

func (api *PubSubAPI) Ls(ctx context.Context) ([]string, error) {
	if err := api.checkNode(); err != nil {
		return nil, err
	}

	return api.node.Floodsub.GetTopics(), nil
}

func (api *PubSubAPI) Peers(ctx context.Context, opts ...caopts.PubSubPeersOption) ([]peer.ID, error) {
	if err := api.checkNode(); err != nil {
		return nil, err
	}

	settings, err := caopts.PubSubPeersOptions(opts...)
	if err != nil {
		return nil, err
	}

	peers := api.node.Floodsub.ListPeers(settings.Topic)
	out := make([]peer.ID, len(peers))

	for i, peer := range peers {
		out[i] = peer
	}

	return out, nil
}

func (api *PubSubAPI) Publish(ctx context.Context, topic string, data []byte) error {
	if err := api.checkNode(); err != nil {
		return err
	}

	return api.node.Floodsub.Publish(topic, data)
}

func (api *PubSubAPI) Subscribe(ctx context.Context, topic string, opts ...caopts.PubSubSubscribeOption) (coreiface.PubSubSubscription, error) {
	options, err := caopts.PubSubSubscribeOptions(opts...)

	if err := api.checkNode(); err != nil {
		return nil, err
	}

	sub, err := api.node.Floodsub.Subscribe(topic)
	if err != nil {
		return nil, err
	}

	pubctx, cancel := context.WithCancel(api.node.Context())

	if options.Discover {
		go func() {
			blk, err := api.core().Block().Put(pubctx, strings.NewReader("floodsub:"+topic))
			if err != nil {
				log.Error("pubsub discovery: ", err)
				return
			}

			connectToPubSubPeers(pubctx, api.node, blk.Path().Cid())
		}()
	}

	return &pubSubSubscription{cancel, sub}, nil
}

func connectToPubSubPeers(ctx context.Context, n *core.IpfsNode, cid cid.Cid) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	provs := n.Routing.FindProvidersAsync(ctx, cid, 10)
	var wg sync.WaitGroup
	for p := range provs {
		wg.Add(1)
		go func(pi pstore.PeerInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			err := n.PeerHost.Connect(ctx, pi)
			if err != nil {
				log.Info("pubsub discover: ", err)
				return
			}
			log.Info("connected to pubsub peer:", pi.ID)
		}(p)
	}

	wg.Wait()
}

func (api *PubSubAPI) checkNode() error {
	if !api.node.OnlineMode() {
		return coreiface.ErrOffline
	}

	if api.node.Floodsub == nil {
		return errors.New("experimental pubsub feature not enabled. Run daemon with --enable-pubsub-experiment to use.")
	}

	return nil
}

func (sub *pubSubSubscription) Close() error {
	sub.cancel()
	sub.subscription.Cancel()
	return nil
}

func (sub *pubSubSubscription) Next(ctx context.Context) (coreiface.PubSubMessage, error) {
	msg, err := sub.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}

	return &pubSubMessage{msg}, nil
}

func (msg *pubSubMessage) From() peer.ID {
	return peer.ID(msg.msg.From)
}

func (msg *pubSubMessage) Data() []byte {
	return msg.msg.Data
}

func (msg *pubSubMessage) Seq() []byte {
	return msg.msg.Seqno
}

func (msg *pubSubMessage) Topics() []string {
	return msg.msg.TopicIDs
}

func (api *PubSubAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
