package p2p

import (
	"context"
	"errors"

	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	net "gx/ipfs/QmPjvxTpVH8qJyQDnxnsxF9kv9jezKD1kozz1hs3fCGsNh/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

var maPrefix = "/" + ma.ProtocolWithCode(ma.P_IPFS).Name + "/"

// remoteListener accepts libp2p streams and proxies them to a manet host
type remoteListener struct {
	p2p *P2P

	// Application proto identifier.
	proto protocol.ID

	// Address to proxy the incoming connections to
	addr ma.Multiaddr

	initialized bool
}

// ForwardRemote creates new p2p listener
func (p2p *P2P) ForwardRemote(ctx context.Context, proto protocol.ID, addr ma.Multiaddr) (Listener, error) {
	listener := &remoteListener{
		p2p: p2p,

		proto: proto,
		addr:  addr,
	}

	if err := p2p.Listeners.Register(listener); err != nil {
		return nil, err
	}

	return listener, nil
}

func (l *remoteListener) start() error {
	// TODO: handle errors when https://github.com/libp2p/go-libp2p-host/issues/16 will be done
	l.p2p.peerHost.SetStreamHandler(l.proto, func(remote net.Stream) {
		local, err := manet.Dial(l.addr)
		if err != nil {
			remote.Reset()
			return
		}

		peerMa, err := ma.NewMultiaddr(maPrefix + remote.Conn().RemotePeer().Pretty())
		if err != nil {
			remote.Reset()
			return
		}

		stream := &Stream{
			Protocol: l.proto,

			OriginAddr: peerMa,
			TargetAddr: l.addr,

			Local:  local,
			Remote: remote,

			Registry: l.p2p.Streams,
		}

		l.p2p.Streams.Register(stream)
		stream.startStreaming()
	})

	l.initialized = true
	return nil
}

func (l *remoteListener) Protocol() protocol.ID {
	return l.proto
}

func (l *remoteListener) ListenAddress() ma.Multiaddr {
	addr, err := ma.NewMultiaddr(maPrefix + l.p2p.identity.Pretty())
	if err != nil {
		panic(err)
	}
	return addr
}

func (l *remoteListener) TargetAddress() ma.Multiaddr {
	return l.addr
}

func (l *remoteListener) Close() error {
	if !l.initialized {
		return errors.New("uninitialized")
	}

	if l.p2p.Listeners.Deregister(getListenerKey(l)) {
		l.p2p.peerHost.RemoveStreamHandler(l.proto)
		l.initialized = false
	}
	return nil
}
