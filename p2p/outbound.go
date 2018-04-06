package p2p

import (
	"context"
	"errors"
	"time"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	net "gx/ipfs/QmYj8wdn5sZEHX2XMDWGBvcXJNdzVbaVpHmXvhHBVZepen/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	pstore "gx/ipfs/QmZb7hAgQEhW9dBbzBudU39gCeD4zbe6xafD52LUuF4cUN/go-libp2p-peerstore"
	manet "gx/ipfs/QmcGXGdw9BWDysPJQHxJinjGHha3eEg4vzFETre4woNwcX/go-multiaddr-net"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
)

// inboundListener accepts libp2p streams and proxies them to a manet host
type outboundListener struct {
	ctx    context.Context
	cancel context.CancelFunc

	p2p *P2P
	id  peer.ID

	proto string
	peer  peer.ID

	listener manet.Listener
}

// Dial creates new P2P stream to a remote listener
func (p2p *P2P) Dial(ctx context.Context, peer peer.ID, proto string, bindAddr ma.Multiaddr) (Listener, error) {
	lnet, _, err := manet.DialArgs(bindAddr)
	if err != nil {
		return nil, err
	}

	switch lnet {
	case "tcp", "tcp4", "tcp6":
		maListener, err := manet.Listen(bindAddr)
		if err != nil {
			return nil, err
		}

		listener := &outboundListener{
			p2p: p2p,
			id:  p2p.identity,

			proto: proto,
			peer:  peer,

			listener: maListener,
		}

		go listener.acceptConns()

		return listener, nil
	default:
		return nil, errors.New("unsupported proto: " + lnet)
	}
}

func (l *outboundListener) dial() (net.Stream, error) {
	ctx, cancel := context.WithTimeout(l.ctx, time.Second*30) //TODO: configurable?
	defer cancel()

	err := l.p2p.peerHost.Connect(ctx, pstore.PeerInfo{ID: l.peer})
	if err != nil {
		return nil, err
	}

	return l.p2p.peerHost.NewStream(l.ctx, l.peer, protocol.ID(l.proto))
}

func (l *outboundListener) acceptConns() {
	for {
		local, err := l.listener.Accept()
		if err != nil {
			return
		}

		remote, err := l.dial()
		if err != nil {
			local.Close()
			return
		}

		stream := StreamInfo{
			Protocol: l.proto,

			LocalPeer: l.id,
			LocalAddr: l.listener.Multiaddr(),

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &l.p2p.Streams,
		}

		l.p2p.Streams.Register(&stream)
		stream.startStreaming()
	}
}

func (l *outboundListener) Close() error {
	l.listener.Close()
	err := l.p2p.Listeners.Deregister(l.proto)
	return err
}

func (l *outboundListener) Protocol() string {
	return l.proto
}

func (l *outboundListener) Address() string {
	return "/ipfs/" + l.peer.String()
}
