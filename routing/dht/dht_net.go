package dht

import (
	"errors"
	"time"

	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	ctxutil "github.com/jbenet/go-ipfs/util/ctx"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
)

// handleNewStream implements the inet.StreamHandler
func (dht *IpfsDHT) handleNewStream(s inet.Stream) {
	go dht.handleNewMessage(s)
}

func (dht *IpfsDHT) handleNewMessage(s inet.Stream) {
	defer s.Close()

	ctx := dht.Context()
	cr := ctxutil.NewReader(ctx, s) // ok to use. we defer close stream in this func
	cw := ctxutil.NewWriter(ctx, s) // ok to use. we defer close stream in this func
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)
	mPeer := s.Conn().RemotePeer()

	// receive msg
	pmes := new(pb.Message)
	if err := r.ReadMsg(pmes); err != nil {
		log.Debugf("Error unmarshaling data: %s", err)
		return
	}

	// update the peer (on valid msgs only)
	dht.updateFromMessage(ctx, mPeer, pmes)

	// get handler for this msg type.
	handler := dht.handlerForMsgType(pmes.GetType())
	if handler == nil {
		log.Debug("got back nil handler from handlerForMsgType")
		return
	}

	// dispatch handler.
	rpmes, err := handler(ctx, mPeer, pmes)
	if err != nil {
		log.Debugf("handle message error: %s", err)
		return
	}

	// if nil response, return it before serializing
	if rpmes == nil {
		log.Warning("Got back nil response from request.")
		return
	}

	// send out response msg
	if err := w.WriteMsg(rpmes); err != nil {
		log.Debugf("send response error: %s", err)
		return
	}

	return
}

// sendRequest sends out a request, but also makes sure to
// measure the RTT for latency measurements.
func (dht *IpfsDHT) sendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {

	log.Debugf("%s dht starting stream", dht.self)
	s, err := dht.host.NewStream(ProtocolDHT, p)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	cr := ctxutil.NewReader(ctx, s) // ok to use. we defer close stream in this func
	cw := ctxutil.NewWriter(ctx, s) // ok to use. we defer close stream in this func
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)

	start := time.Now()

	if err := w.WriteMsg(pmes); err != nil {
		return nil, err
	}
	log.Event(ctx, "dhtSentMessage", dht.self, p, pmes)

	rpmes := new(pb.Message)
	if err := r.ReadMsg(rpmes); err != nil {
		return nil, err
	}
	if rpmes == nil {
		return nil, errors.New("no response to request")
	}

	// update the peer (on valid msgs only)
	dht.updateFromMessage(ctx, p, rpmes)

	dht.peerstore.RecordLatency(p, time.Since(start))
	log.Event(ctx, "dhtReceivedMessage", dht.self, p, rpmes)
	return rpmes, nil
}

// sendMessage sends out a message
func (dht *IpfsDHT) sendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error {

	log.Debugf("%s dht starting stream", dht.self)
	s, err := dht.host.NewStream(ProtocolDHT, p)
	if err != nil {
		return err
	}
	defer s.Close()

	cw := ctxutil.NewWriter(ctx, s) // ok to use. we defer close stream in this func
	w := ggio.NewDelimitedWriter(cw)

	if err := w.WriteMsg(pmes); err != nil {
		return err
	}
	log.Event(ctx, "dhtSentMessage", dht.self, p, pmes)
	return nil
}

func (dht *IpfsDHT) updateFromMessage(ctx context.Context, p peer.ID, mes *pb.Message) error {
	dht.Update(ctx, p)
	return nil
}
