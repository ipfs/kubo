package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
)

// PubSubSubscription is an active PubSub subscription
type PubSubSubscription interface {
	io.Closer

	// Next return the next incoming message
	Next(context.Context) (PubSubMessage, error)
}

// PubSubMessage is a single PubSub message
type PubSubMessage interface {
	// From returns id of a peer from which the message has arrived
	From() peer.ID

	// Data returns the message body
	Data() []byte
}

// PubSubAPI specifies the interface to PubSub
type PubSubAPI interface {
	// Ls lists subscribed topics by name
	Ls(context.Context) ([]string, error)

	// Peers list peers we are currently pubsubbing with
	// TODO: WithTopic
	Peers(context.Context, ...options.PubSubPeersOption) ([]peer.ID, error)

	// Publish a message to a given pubsub topic
	Publish(context.Context, string, []byte) error

	// Subscribe to messages on a given topic
	Subscribe(context.Context, string, ...options.PubSubSubscribeOption) (PubSubSubscription, error)
}
