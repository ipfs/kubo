package corenet

import (
	"time"

	core "github.com/ipfs/go-ipfs/core"
	net "gx/ipfs/QmQGXmfLoXHxw8CBH2EBxiicWHNwMw9qXQdsRDd9UejMB4/go-libp2p/p2p/net"
	pro "gx/ipfs/QmQGXmfLoXHxw8CBH2EBxiicWHNwMw9qXQdsRDd9UejMB4/go-libp2p/p2p/protocol"
	peer "gx/ipfs/QmTLeWzYxCE3G4TpArXpHSaa8xJqjm6JnxJdNSSDedJ2if/go-libp2p-peer"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	pstore "gx/ipfs/Qmax7ca82J81QamkGgEG2zqWj35B9QHrDgywzKb8nm8Twp/go-libp2p-peerstore"
)

type ipfsListener struct {
	conCh  chan net.Stream
	proto  pro.ID
	ctx    context.Context
	cancel func()
}

func (il *ipfsListener) Accept() (net.Stream, error) {
	select {
	case c := <-il.conCh:
		return c, nil
	case <-il.ctx.Done():
		return nil, il.ctx.Err()
	}
}

func (il *ipfsListener) Close() error {
	il.cancel()
	// TODO: unregister handler from peerhost
	return nil
}

func Listen(nd *core.IpfsNode, protocol string) (*ipfsListener, error) {
	ctx, cancel := context.WithCancel(nd.Context())

	list := &ipfsListener{
		proto:  pro.ID(protocol),
		conCh:  make(chan net.Stream),
		ctx:    ctx,
		cancel: cancel,
	}

	nd.PeerHost.SetStreamHandler(list.proto, func(s net.Stream) {
		select {
		case list.conCh <- s:
		case <-ctx.Done():
			s.Close()
		}
	})

	return list, nil
}

func Dial(nd *core.IpfsNode, p peer.ID, protocol string) (net.Stream, error) {
	ctx, cancel := context.WithTimeout(nd.Context(), time.Second*30)
	defer cancel()
	err := nd.PeerHost.Connect(ctx, pstore.PeerInfo{ID: p})
	if err != nil {
		return nil, err
	}
	return nd.PeerHost.NewStream(nd.Context(), pro.ID(protocol), p)
}
