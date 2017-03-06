package proxy

import (
	context "context"
	inet "gx/ipfs/QmVtMT3fD7DzQNW7hdm6Xe6KPstzcggrhNpeVZ4422UpKK/go-libp2p-net"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	dhtpb "gx/ipfs/QmaoxFZcgwGyoB57pCYQobejLoNgqaA6trr3zxxrbm4UXe/go-libp2p-kad-dht/pb"
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
