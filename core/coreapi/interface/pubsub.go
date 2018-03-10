package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// PubSubSubscription is an active PubSub subscription
type PubSubSubscription interface {
	io.Closer

	// Chan return incoming message channel
	Chan(context.Context) <-chan PubSubMessage
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

	// WithTopic is an option for peers which specifies a topic filter for the
	// function
	WithTopic(topic string) options.PubSubPeersOption

	// Publish a message to a given pubsub topic
	Publish(context.Context, string, []byte) error

	// Subscribe to messages on a given topic
	Subscribe(context.Context, string) (PubSubSubscription, error)

	// WithDiscover is an option for Subscribe which specifies whether to try to
	// discover other peers subscribed to the same topic
	WithDiscover(discover bool) options.PubSubSubscribeOption
}
