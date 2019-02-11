package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-peer"
)

type PubsubAPI HttpApi

func (api *PubsubAPI) Ls(ctx context.Context) ([]string, error) {
	var out struct {
		Strings []string
	}

	if err := api.core().request("pubsub/ls").Exec(ctx, &out); err != nil {
		return nil, err
	}

	return out.Strings, nil
}

func (api *PubsubAPI) Peers(ctx context.Context, opts ...caopts.PubSubPeersOption) ([]peer.ID, error) {
	options, err := caopts.PubSubPeersOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out struct {
		Strings []string
	}

	if err := api.core().request("pubsub/peers", options.Topic).Exec(ctx, &out); err != nil {
		return nil, err
	}

	res := make([]peer.ID, len(out.Strings))
	for i, sid := range out.Strings {
		id, err := peer.IDB58Decode(sid)
		if err != nil {
			return nil, err
		}
		res[i] = id
	}
	return res, nil
}

func (api *PubsubAPI) Publish(ctx context.Context, topic string, message []byte) error {
	return api.core().request("pubsub/pub", topic).
		FileBody(bytes.NewReader(message)).
		Exec(ctx, nil)
}

type pubsubSub struct {
	io.Closer
	dec *json.Decoder
}

type pubsubMessage struct {
	JFrom     []byte   `json:"from,omitempty"`
	JData     []byte   `json:"data,omitempty"`
	JSeqno    []byte   `json:"seqno,omitempty"`
	JTopicIDs []string `json:"topicIDs,omitempty"`
}

func (msg *pubsubMessage) valid() error {
	_, err := peer.IDFromBytes(msg.JFrom)
	return err
}

func (msg *pubsubMessage) From() peer.ID {
	id, _ := peer.IDFromBytes(msg.JFrom)
	return id
}

func (msg *pubsubMessage) Data() []byte {
	return msg.JData
}

func (msg *pubsubMessage) Seq() []byte {
	return msg.JSeqno
}

func (msg *pubsubMessage) Topics() []string {
	return msg.JTopicIDs
}

func (s *pubsubSub) Next(ctx context.Context) (iface.PubSubMessage, error) {
	// TODO: handle ctx

	var msg pubsubMessage
	if err := s.dec.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, msg.valid()
}

func (api *PubsubAPI) Subscribe(ctx context.Context, topic string, opts ...caopts.PubSubSubscribeOption) (iface.PubSubSubscription, error) {
	options, err := caopts.PubSubSubscribeOptions(opts...)
	if err != nil {
		return nil, err
	}

	resp, err := api.core().request("pubsub/sub", topic).
		Option("discover", options.Discover).
		Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	return &pubsubSub{
		Closer: resp,
		dec:    json.NewDecoder(resp.Output),
	}, nil
}

func (api *PubsubAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
