package proxy

import (
	peer "gx/ipfs/QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p/go-libp2p-peer"
	dhtpb "gx/ipfs/QmXVWh4XWRaRGdcGeFtBp3hx7H3mzYgVRpVN7LwtTaYv2E/go-libp2p-kad-dht/pb"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	inet "gx/ipfs/QmbiRCGZqhfcSjnm9icGz3oNQQdPLAnLWnKHXixaEWXVCN/go-libp2p/p2p/net"
)

// RequestHandler handles routing requests locally
type RequestHandler interface {
	HandleRequest(ctx context.Context, p peer.ID, m *dhtpb.Message) *dhtpb.Message
}

// Loopback forwards requests to a local handler
type Loopback struct {
	Handler RequestHandler
	Local   peer.ID
}

func (_ *Loopback) Bootstrap(ctx context.Context) error {
	return nil
}

// SendMessage intercepts local requests, forwarding them to a local handler
func (lb *Loopback) SendMessage(ctx context.Context, m *dhtpb.Message) error {
	response := lb.Handler.HandleRequest(ctx, lb.Local, m)
	if response != nil {
		log.Warning("loopback handler returned unexpected message")
	}
	return nil
}

// SendRequest intercepts local requests, forwarding them to a local handler
func (lb *Loopback) SendRequest(ctx context.Context, m *dhtpb.Message) (*dhtpb.Message, error) {
	return lb.Handler.HandleRequest(ctx, lb.Local, m), nil
}

func (lb *Loopback) HandleStream(s inet.Stream) {
	defer s.Close()
	pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
	var incoming dhtpb.Message
	if err := pbr.ReadMsg(&incoming); err != nil {
		log.Debug(err)
		return
	}
	ctx := context.TODO()
	outgoing := lb.Handler.HandleRequest(ctx, s.Conn().RemotePeer(), &incoming)

	pbw := ggio.NewDelimitedWriter(s)

	if err := pbw.WriteMsg(outgoing); err != nil {
		return // TODO logerr
	}
}
