package coreapi

import (
	"context"
	"errors"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
)

type PubSubAPI struct {
	*CoreAPI
	*caopts.PubSubOptions
}

type pubSubSubscription struct {
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

func (api *PubSubAPI) Peers(ctx context.Context, opts ...caopts.PubSubPeersOption) ([]coreiface.PeerID, error) {
	if err := api.checkNode(); err != nil {
		return nil, err
	}

	settings, err := caopts.PubSubPeersOptions(opts...)
	if err != nil {
		return nil, err
	}

	peers := api.node.Floodsub.ListPeers(settings.Topic)
	out := make([]coreiface.PeerID, len(peers))

	for i, peer := range peers {
		out[i] = coreiface.PeerID(peer)
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
	if err := api.checkNode(); err != nil {
		return nil, err
	}

	sub, err := api.node.Floodsub.Subscribe(topic)
	if err != nil {
		return nil, err
	}

	return &pubSubSubscription{sub}, nil
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

func (msg *pubSubMessage) From() coreiface.PeerID {
	return coreiface.PeerID(msg.msg.From)
}

func (msg *pubSubMessage) Data() []byte {
	return msg.msg.Data
}
