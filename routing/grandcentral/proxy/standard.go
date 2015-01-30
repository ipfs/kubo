package proxy

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dhtpb "github.com/jbenet/go-ipfs/routing/dht/pb"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

var log = eventlog.Logger("proxy")

const ProtocolGCR = "/ipfs/grandcentral"

type Proxy interface {
	HandleStream(inet.Stream)
	SendMessage(ctx context.Context, m *dhtpb.Message) error
	SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error)
}

type standard struct {
	Host    host.Host
	Remotes []peer.ID
}

func Standard(h host.Host, remotes []peer.ID) Proxy {
	return &standard{h, remotes}
}

func (p *standard) HandleStream(s inet.Stream) {
	panic("client received a GCR message")
}

func (px *standard) SendMessage(ctx context.Context, m *dhtpb.Message) error {
	var err error
	for _, remote := range px.Remotes {
		if err = px.sendMessage(ctx, m, remote); err != nil { // careful don't re-declare err!
			continue
		}
		return nil // success
	}
	return err // NB: returns the last error
}

func (px *standard) sendMessage(ctx context.Context, m *dhtpb.Message, remote peer.ID) error {
	if err := px.Host.Connect(ctx, peer.PeerInfo{ID: remote}); err != nil {
		return err
	}
	s, err := px.Host.NewStream(ProtocolGCR, remote)
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
	var err error
	for _, remote := range px.Remotes {
		var reply *dhtpb.Message
		reply, err = px.sendRequest(ctx, m, remote) // careful don't redeclare err!
		if err != nil {
			continue
		}
		return reply, nil // success
	}
	return nil, err // NB: returns the last error
}

func (px *standard) sendRequest(ctx context.Context, m *dhtpb.Message, remote peer.ID) (*dhtpb.Message, error) {
	if err := px.Host.Connect(ctx, peer.PeerInfo{ID: remote}); err != nil {
		return nil, err
	}
	s, err := px.Host.NewStream(ProtocolGCR, remote)
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
