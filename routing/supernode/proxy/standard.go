package proxy

import (
	"math/rand"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dhtpb "github.com/jbenet/go-ipfs/routing/dht/pb"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

const ProtocolSNR = "/ipfs/supernoderouting"

var log = eventlog.Logger("supernode/proxy")

type Proxy interface {
	Bootstrap(context.Context) error
	HandleStream(inet.Stream)
	SendMessage(ctx context.Context, m *dhtpb.Message) error
	SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error)
}

type standard struct {
	Host    host.Host
	Remotes []peer.PeerInfo
}

func Standard(h host.Host, remotes []peer.PeerInfo) Proxy {
	return &standard{h, remotes}
}

func (px *standard) Bootstrap(ctx context.Context) error {
	var cxns []peer.PeerInfo
	for _, info := range px.Remotes {
		if err := px.Host.Connect(ctx, info); err != nil {
			continue
		}
		cxns = append(cxns, info)
	}
	if len(cxns) == 0 {
		log.Critical("unable to bootstrap to any supernode routers")
	} else {
		log.Info("bootstrapped to %d supernode routers: %s", len(cxns), cxns)
	}
	return nil
}

func (p *standard) HandleStream(s inet.Stream) {
	// TODO(brian): Should clients be able to satisfy requests?
	log.Error("supernode client received (dropped) a routing message from", s.Conn().RemotePeer())
	s.Close()
}

// SendMessage sends message to each remote sequentially (randomized order),
// stopping after the first successful response. If all fail, returns the last
// error.
func (px *standard) SendMessage(ctx context.Context, m *dhtpb.Message) error {
	var err error
	for _, i := range rand.Perm(len(px.Remotes)) {
		remote := px.Remotes[i].ID
		if err = px.sendMessage(ctx, m, remote); err != nil { // careful don't re-declare err!
			continue
		}
		return nil // success
	}
	return err // NB: returns the last error
}

func (px *standard) sendMessage(ctx context.Context, m *dhtpb.Message, remote peer.ID) (err error) {
	e := log.EventBegin(ctx, "sendRoutingMessage", px.Host.ID(), remote, m)
	defer func() {
		if err != nil {
			e.SetError(err)
		}
		e.Done()
	}()
	if err = px.Host.Connect(ctx, peer.PeerInfo{ID: remote}); err != nil {
		return err
	}
	s, err := px.Host.NewStream(ProtocolSNR, remote)
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

// SendRequest sends the request to each remote sequentially (randomized order),
// stopping after the first successful response. If all fail, returns the last
// error.
func (px *standard) SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error) {
	var err error
	for _, i := range rand.Perm(len(px.Remotes)) {
		remote := px.Remotes[i].ID
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
	e := log.EventBegin(ctx, "sendRoutingRequest", px.Host.ID(), remote, eventlog.Pair("request", m))
	defer e.Done()
	if err := px.Host.Connect(ctx, peer.PeerInfo{ID: remote}); err != nil {
		e.SetError(err)
		return nil, err
	}
	s, err := px.Host.NewStream(ProtocolSNR, remote)
	if err != nil {
		e.SetError(err)
		return nil, err
	}
	defer s.Close()
	r := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(s)
	if err = w.WriteMsg(m); err != nil {
		e.SetError(err)
		return nil, err
	}

	response := &dhtpb.Message{}
	if err = r.ReadMsg(response); err != nil {
		e.SetError(err)
		return nil, err
	}
	// need ctx expiration?
	if response == nil {
		err := errors.New("no response to request")
		e.SetError(err)
		return nil, err
	}
	e.Append(eventlog.Pair("response", response))
	e.Append(eventlog.Pair("uuid", eventlog.Uuid("foo")))
	return response, nil
}
