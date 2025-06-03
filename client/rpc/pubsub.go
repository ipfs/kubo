package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	iface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
)

type PubsubAPI HttpApi

func (api *PubsubAPI) Ls(ctx context.Context) ([]string, error) {
	var out struct {
		Strings []string
	}

	if err := api.core().Request("pubsub/ls").Exec(ctx, &out); err != nil {
		return nil, err
	}
	topics := make([]string, len(out.Strings))
	for n, mb := range out.Strings {
		_, topic, err := mbase.Decode(mb)
		if err != nil {
			return nil, err
		}
		topics[n] = string(topic)
	}
	return topics, nil
}

func (api *PubsubAPI) Peers(ctx context.Context, opts ...caopts.PubSubPeersOption) ([]peer.ID, error) {
	options, err := caopts.PubSubPeersOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out struct {
		Strings []string
	}

	var optionalTopic string
	if len(options.Topic) > 0 {
		optionalTopic = toMultibase([]byte(options.Topic))
	}
	if err := api.core().Request("pubsub/peers", optionalTopic).Exec(ctx, &out); err != nil {
		return nil, err
	}

	res := make([]peer.ID, len(out.Strings))
	for i, sid := range out.Strings {
		id, err := peer.Decode(sid)
		if err != nil {
			return nil, err
		}
		res[i] = id
	}
	return res, nil
}

func (api *PubsubAPI) Publish(ctx context.Context, topic string, message []byte) error {
	return api.core().Request("pubsub/pub", toMultibase([]byte(topic))).
		FileBody(bytes.NewReader(message)).
		Exec(ctx, nil)
}

type pubsubSub struct {
	messages chan pubsubMessage

	done    chan struct{}
	rcloser func() error
}

type pubsubMessage struct {
	JFrom     string   `json:"from,omitempty"`
	JData     string   `json:"data,omitempty"`
	JSeqno    string   `json:"seqno,omitempty"`
	JTopicIDs []string `json:"topicIDs,omitempty"`

	// real values after unpacking from text/multibase envelopes
	from   peer.ID
	data   []byte
	seqno  []byte
	topics []string

	err error
}

func (msg *pubsubMessage) From() peer.ID {
	return msg.from
}

func (msg *pubsubMessage) Data() []byte {
	return msg.data
}

func (msg *pubsubMessage) Seq() []byte {
	return msg.seqno
}

// TODO: do we want to keep this interface as []string,
// or change to more correct [][]byte?
func (msg *pubsubMessage) Topics() []string {
	return msg.topics
}

func (s *pubsubSub) Next(ctx context.Context) (iface.PubSubMessage, error) {
	select {
	case msg, ok := <-s.messages:
		if !ok {
			return nil, io.EOF
		}
		if msg.err != nil {
			return nil, msg.err
		}
		// unpack values from text/multibase envelopes
		var err error
		msg.from, err = peer.Decode(msg.JFrom)
		if err != nil {
			return nil, err
		}
		_, msg.data, err = mbase.Decode(msg.JData)
		if err != nil {
			return nil, err
		}
		_, msg.seqno, err = mbase.Decode(msg.JSeqno)
		if err != nil {
			return nil, err
		}
		for _, mbt := range msg.JTopicIDs {
			_, topic, err := mbase.Decode(mbt)
			if err != nil {
				return nil, err
			}
			msg.topics = append(msg.topics, string(topic))
		}
		return &msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (api *PubsubAPI) Subscribe(ctx context.Context, topic string, opts ...caopts.PubSubSubscribeOption) (iface.PubSubSubscription, error) {
	/* right now we have no options (discover got deprecated)
	options, err := caopts.PubSubSubscribeOptions(opts...)
	if err != nil {
		return nil, err
	}
	*/
	resp, err := api.core().Request("pubsub/sub", toMultibase([]byte(topic))).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	sub := &pubsubSub{
		messages: make(chan pubsubMessage),
		done:     make(chan struct{}),
		rcloser: func() error {
			return resp.Cancel()
		},
	}

	dec := json.NewDecoder(resp.Output)

	go func() {
		defer close(sub.messages)

		for {
			var msg pubsubMessage
			if err := dec.Decode(&msg); err != nil {
				if err == io.EOF {
					return
				}
				msg.err = err
			}

			select {
			case sub.messages <- msg:
			case <-sub.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return sub, nil
}

func (s *pubsubSub) Close() error {
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
	return s.rcloser()
}

func (api *PubsubAPI) core() *HttpApi {
	return (*HttpApi)(api)
}

// Encodes bytes into URL-safe multibase that can be sent over HTTP RPC (URL or body).
func toMultibase(data []byte) string {
	mb, _ := mbase.Encode(mbase.Base64url, data)
	return mb
}
