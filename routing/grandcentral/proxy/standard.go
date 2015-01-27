package proxy

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	host "github.com/jbenet/go-ipfs/p2p/host"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dhtpb "github.com/jbenet/go-ipfs/routing/dht/pb"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("proxy")

type Proxy interface {
	SendMessage(ctx context.Context, m *dhtpb.Message) error
	SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error)
}

type standard struct {
	Host   host.Host
	Remote peer.ID
}

func Standard(h host.Host, remote peer.ID) Proxy {
	return &standard{h, remote}
}

const ProtocolGCR = "/ipfs/grandcentral"

func (px *standard) SendMessage(ctx context.Context, m *dhtpb.Message) error {
	if err := px.Host.Connect(ctx, peer.PeerInfo{ID: px.Remote}); err != nil {
		return err
	}
	s, err := px.Host.NewStream(ProtocolGCR, px.Remote)
	if err != nil {
		return err
	}
	defer s.Close()
	pbw := ggio.NewDelimitedWriter(s)
	if err := pbw.WriteMsg(m); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (px *standard) SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error) {
	if err := px.Host.Connect(ctx, peer.PeerInfo{ID: px.Remote}); err != nil {
		return nil, err
	}
	s, err := px.Host.NewStream(ProtocolGCR, px.Remote)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	r := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(s)
	if err := w.WriteMsg(m); err != nil {
		return nil, err
	}

	var reply dhtpb.Message
	if err := r.ReadMsg(&reply); err != nil {
		return nil, err
	}
	// need ctx expiration?
	if &reply == nil {
		return nil, errors.New("no response to request")
	}
	return &reply, nil
}

