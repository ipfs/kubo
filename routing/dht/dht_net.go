package dht

import (
	"sync"
	"time"

	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"
	inet "gx/ipfs/QmQgQeBQxQmJdeUSaDagc8cr2ompDwGn13Cybjdtzfuaki/go-libp2p/p2p/net"
	ctxio "gx/ipfs/QmX6DhWrpBB5NtadXmPSXYNdVvuLfJXoFNMvUMoVvP5UJa/go-context/io"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// handleNewStream implements the inet.StreamHandler
func (dht *IpfsDHT) handleNewStream(s inet.Stream) {
	go dht.handleNewMessage(s)
}

func (dht *IpfsDHT) handleNewMessage(s inet.Stream) {
	defer s.Close()

	ctx := dht.Context()
	cr := ctxio.NewReader(ctx, s) // ok to use. we defer close stream in this func
	cw := ctxio.NewWriter(ctx, s) // ok to use. we defer close stream in this func
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)
	mPeer := s.Conn().RemotePeer()

	for {
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
			log.Debug("Got back nil response from request.")
			continue
		}

		// send out response msg
		if err := w.WriteMsg(rpmes); err != nil {
			log.Debugf("send response error: %s", err)
			return
		}
	}

	return
}

// sendRequest sends out a request, but also makes sure to
// measure the RTT for latency measurements.
func (dht *IpfsDHT) sendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {

	ms := dht.messageSenderForPeer(p)

	start := time.Now()

	rpmes, err := ms.SendRequest(ctx, pmes)
	if err != nil {
		return nil, err
	}

	// update the peer (on valid msgs only)
	dht.updateFromMessage(ctx, p, rpmes)

	dht.peerstore.RecordLatency(p, time.Since(start))
	log.Event(ctx, "dhtReceivedMessage", dht.self, p, rpmes)
	return rpmes, nil
}

// sendMessage sends out a message
func (dht *IpfsDHT) sendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error {

	ms := dht.messageSenderForPeer(p)

	if err := ms.SendMessage(ctx, pmes); err != nil {
		return err
	}
	log.Event(ctx, "dhtSentMessage", dht.self, p, pmes)
	return nil
}

func (dht *IpfsDHT) updateFromMessage(ctx context.Context, p peer.ID, mes *pb.Message) error {
	dht.Update(ctx, p)
	return nil
}

func (dht *IpfsDHT) messageSenderForPeer(p peer.ID) *messageSender {
	dht.smlk.Lock()
	defer dht.smlk.Unlock()

	ms, ok := dht.strmap[p]
	if !ok {
		ms = dht.newMessageSender(p)
		dht.strmap[p] = ms
	}

	return ms
}

type messageSender struct {
	s   inet.Stream
	r   ggio.ReadCloser
	w   ggio.WriteCloser
	lk  sync.Mutex
	p   peer.ID
	dht *IpfsDHT
}

func (dht *IpfsDHT) newMessageSender(p peer.ID) *messageSender {
	return &messageSender{p: p, dht: dht}
}

func (ms *messageSender) prep() error {
	if ms.s != nil {
		return nil
	}

	nstr, err := ms.dht.host.NewStream(ms.dht.ctx, ProtocolDHT, ms.p)
	if err != nil {
		return err
	}

	ms.r = ggio.NewDelimitedReader(nstr, inet.MessageSizeMax)
	ms.w = ggio.NewDelimitedWriter(nstr)
	ms.s = nstr

	return nil
}

func (ms *messageSender) SendMessage(ctx context.Context, pmes *pb.Message) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(); err != nil {
		return err
	}

	err := ms.w.WriteMsg(pmes)
	if err != nil {
		ms.s.Close()
		ms.s = nil
		return err
	}
	return nil
}

func (ms *messageSender) SendRequest(ctx context.Context, pmes *pb.Message) (*pb.Message, error) {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(); err != nil {
		return nil, err
	}

	err := ms.w.WriteMsg(pmes)
	if err != nil {
		ms.s.Close()
		ms.s = nil
		return nil, err
	}

	log.Event(ctx, "dhtSentMessage", ms.dht.self, ms.p, pmes)

	mes := new(pb.Message)
	err = ms.r.ReadMsg(mes)
	if err != nil {
		ms.s.Close()
		ms.s = nil
		return nil, err
	}

	return mes, nil
}
