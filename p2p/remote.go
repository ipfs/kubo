package p2p

import (
	"context"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	net "gx/ipfs/QmYj8wdn5sZEHX2XMDWGBvcXJNdzVbaVpHmXvhHBVZepen/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	manet "gx/ipfs/QmcGXGdw9BWDysPJQHxJinjGHha3eEg4vzFETre4woNwcX/go-multiaddr-net"
)

// remoteListener accepts libp2p streams and proxies them to a manet host
type remoteListener struct {
	p2p *P2P

	// Application proto identifier.
	proto string

	// Address to proxy the incoming connections to
	addr ma.Multiaddr
}

// ForwardRemote creates new p2p listener
func (p2p *P2P) ForwardRemote(ctx context.Context, proto string, addr ma.Multiaddr) (Listener, error) {
	listenerInfo := &remoteListener{
		p2p: p2p,

		proto: proto,
		addr:  addr,
	}

	p2p.Listeners.Register(listenerInfo)

	p2p.peerHost.SetStreamHandler(protocol.ID(proto), func(remote net.Stream) {
		local, err := manet.Dial(addr)
		if err != nil {
			remote.Reset()
			return
		}

		stream := Stream{
			Protocol: proto,

			OriginAddr: remote.Conn().RemoteMultiaddr(),
			TargetAddr: addr,

			Local:  local,
			Remote: remote,

			Registry: p2p.Streams,
		}

		p2p.Streams.Register(&stream)
		stream.startStreaming()
	})

	return listenerInfo, nil
}

func (l *remoteListener) Protocol() string {
	return l.proto
}

func (l *remoteListener) ListenAddress() string {
	return "/ipfs"
}

func (l *remoteListener) TargetAddress() string {
	return l.addr.String()
}

func (l *remoteListener) Close() error {
	l.p2p.peerHost.RemoveStreamHandler(protocol.ID(l.proto))
	l.p2p.Listeners.Deregister(getListenerKey(l))
	return nil
}
