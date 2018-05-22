package p2p

import (
	"context"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	net "gx/ipfs/QmYj8wdn5sZEHX2XMDWGBvcXJNdzVbaVpHmXvhHBVZepen/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	manet "gx/ipfs/QmcGXGdw9BWDysPJQHxJinjGHha3eEg4vzFETre4woNwcX/go-multiaddr-net"
)

// inboundListener accepts libp2p streams and proxies them to a manet host
type inboundListener struct {
	p2p *P2P

	// Application proto identifier.
	proto string

	// Address to proxy the incoming connections to
	addr ma.Multiaddr
}

// NewListener creates new p2p listener
func (p2p *P2P) NewListener(ctx context.Context, proto string, addr ma.Multiaddr) (Listener, error) {
	listenerInfo := &inboundListener{
		p2p: p2p,

		proto: proto,
		addr:  addr,
	}

	p2p.peerHost.SetStreamHandler(protocol.ID(proto), func(remote net.Stream) {
		local, err := manet.Dial(addr)
		if err != nil {
			remote.Reset()
			return
		}

		stream := Stream{
			Protocol: proto,

			LocalPeer: p2p.identity,
			LocalAddr: addr,

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &p2p.Streams,
		}

		p2p.Streams.Register(&stream)
		stream.startStreaming()
	})

	p2p.Listeners.Register(listenerInfo)

	return listenerInfo, nil
}

func (l *inboundListener) Protocol() string {
	return l.proto
}

func (l *inboundListener) Address() string {
	return l.addr.String()
}

func (l *inboundListener) Close() error {
	l.p2p.peerHost.RemoveStreamHandler(protocol.ID(l.proto))
	l.p2p.Listeners.Deregister(l.proto)
	return nil
}
