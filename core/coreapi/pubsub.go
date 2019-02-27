package coreapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	pubsub "gx/ipfs/QmVzLBPPg4gdyX3XFnNaNDkK4V81ptT5X6WZVFzTUECXMa/go-libp2p-pubsub"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	caopts "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	p2phost "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
)

type PubSubAPI CoreAPI

type pubSubSubscription struct {
	cancel       context.CancelFunc
	subscription *pubsub.Subscription
}

type pubSubMessage struct {
	msg *pubsub.Message
}

func (api *PubSubAPI) Ls(ctx context.Context) ([]string, error) {
	_, err := api.checkNode()
	if err != nil {
		return nil, err
	}

	return api.pubSub.GetTopics(), nil
}

func (api *PubSubAPI) Peers(ctx context.Context, opts ...caopts.PubSubPeersOption) ([]peer.ID, error) {
	_, err := api.checkNode()
	if err != nil {
		return nil, err
	}

	settings, err := caopts.PubSubPeersOptions(opts...)
	if err != nil {
		return nil, err
	}

	peers := api.pubSub.ListPeers(settings.Topic)
	out := make([]peer.ID, len(peers))

	for i, peer := range peers {
		out[i] = peer
	}

	return out, nil
}

func (api *PubSubAPI) Publish(ctx context.Context, topic string, data []byte) error {
	_, err := api.checkNode()
	if err != nil {
		return err
	}

	return api.pubSub.Publish(topic, data)
}

func (api *PubSubAPI) Subscribe(ctx context.Context, topic string, opts ...caopts.PubSubSubscribeOption) (coreiface.PubSubSubscription, error) {
	options, err := caopts.PubSubSubscribeOptions(opts...)

	r, err := api.checkNode()
	if err != nil {
		return nil, err
	}

	sub, err := api.pubSub.Subscribe(topic)
	if err != nil {
		return nil, err
	}

	pubctx, cancel := context.WithCancel(api.nctx)

	if options.Discover {
		go func() {
			blk, err := api.core().Block().Put(pubctx, strings.NewReader("floodsub:"+topic))
			if err != nil {
				log.Error("pubsub discovery: ", err)
				return
			}

			connectToPubSubPeers(pubctx, r, api.peerHost, blk.Path().Cid())
		}()
	}

	return &pubSubSubscription{cancel, sub}, nil
}

func connectToPubSubPeers(ctx context.Context, r routing.IpfsRouting, ph p2phost.Host, cid cid.Cid) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	provs := r.FindProvidersAsync(ctx, cid, 10)
	var wg sync.WaitGroup
	for p := range provs {
		wg.Add(1)
		go func(pi pstore.PeerInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			err := ph.Connect(ctx, pi)
			if err != nil {
				log.Info("pubsub discover: ", err)
				return
			}
			log.Info("connected to pubsub peer:", pi.ID)
		}(p)
	}

	wg.Wait()
}

func (api *PubSubAPI) checkNode() (routing.IpfsRouting, error) {
	if api.pubSub == nil {
		return nil, errors.New("experimental pubsub feature not enabled. Run daemon with --enable-pubsub-experiment to use.")
	}

	err := api.checkOnline(false)
	if err != nil {
		return nil, err
	}

	return api.routing, nil
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
