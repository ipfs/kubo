package corenet

import (
	"time"

	core "github.com/ipfs/go-ipfs/core"
	peer "github.com/ipfs/go-libp2p-peer"
	pstore "github.com/ipfs/go-libp2p-peerstore"
	net "github.com/libp2p/go-libp2p/p2p/net"
	pro "github.com/libp2p/go-libp2p/p2p/protocol"
	context "golang.org/x/net/context"
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
	return nd.PeerHost.NewStream(nd.Context(), p, pro.ID(protocol))
}
