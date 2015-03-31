package corenet

import (
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	net "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	pro "github.com/ipfs/go-ipfs/p2p/protocol"
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
	err := nd.PeerHost.Connect(ctx, peer.PeerInfo{ID: p})
	if err != nil {
		return nil, err
	}
	return nd.PeerHost.NewStream(pro.ID(protocol), p)
}
