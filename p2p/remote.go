package p2p

import (
	"context"

	manet "gx/ipfs/QmNqRnejxJxjRroz7buhrjfU8i3yNBLa81hFtmf2pXEffN/go-multiaddr-net"
	ma "gx/ipfs/QmUxSEGbv2nmYNnfXi7839wwQqTN3kwQeUxe8dTjZWZs7J/go-multiaddr"
	net "gx/ipfs/QmXdgNhVEgjLxjUoMs5ViQL7pboAt3Y7V7eGHRiE4qrmTE/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

// remoteListener accepts libp2p streams and proxies them to a manet host
type remoteListener struct {
	p2p *P2P

	// Application proto identifier.
	proto protocol.ID

	// Address to proxy the incoming connections to
	addr ma.Multiaddr
}

// ForwardRemote creates new p2p listener
func (p2p *P2P) ForwardRemote(ctx context.Context, proto string, addr ma.Multiaddr) (Listener, error) {
	listener := &remoteListener{
		p2p: p2p,

		proto: protocol.ID(proto),
		addr:  addr,
	}

	if err := p2p.Listeners.lock(listener); err != nil {
		return nil, err
	}

	p2p.peerHost.SetStreamHandler(listener.proto, func(remote net.Stream) {
		local, err := manet.Dial(addr)
		if err != nil {
			remote.Reset()
			return
		}

		//TODO: review: is there a better way to do this?
		peerMa, err := ma.NewMultiaddr("/ipfs/" + remote.Conn().RemotePeer().Pretty())
		if err != nil {
			remote.Reset()
			return
		}

		stream := &Stream{
			Protocol: listener.proto,

			OriginAddr: peerMa,
			TargetAddr: addr,

			Local:  local,
			Remote: remote,

			Registry: p2p.Streams,
		}

		p2p.Streams.Register(stream)
		stream.startStreaming()
	})

	p2p.Listeners.Register(listener)

	return listener, nil
}

func (l *remoteListener) Protocol() string {
	return string(l.proto)
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
