package p2p

import (
	"context"

	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
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
}

// ForwardRemote creates new p2p listener
func (p2p *P2P) ForwardRemote(ctx context.Context, proto protocol.ID, addr ma.Multiaddr) (P2PListener, error) {
	listener := &remoteListener{
		p2p: p2p,

		proto: proto,
		addr:  addr,
	}

	if err := p2p.ListenersP2P.Register(listener); err != nil {
		return nil, err
	}

	return listener, nil
}

func (l *remoteListener) start() error {
	return nil
}

func (l *remoteListener) handleStream(remote net.Stream) {
	local, err := manet.Dial(l.addr)
	if err != nil {
		remote.Reset()
		return
	}

	peer := remote.Conn().RemotePeer()

	peerMa, err := ma.NewMultiaddr(maPrefix + peer.Pretty())
	if err != nil {
		remote.Reset()
		return
	}

	cmgr := l.p2p.peerHost.ConnManager()
	cmgr.TagPeer(peer, CMGR_TAG, 20)

	stream := &Stream{
		Protocol: l.proto,

		OriginAddr: peerMa,
		TargetAddr: l.addr,

		Local:  local,
		Remote: remote,

		Registry: l.p2p.Streams,

		cleanup: func() {
			cmgr.UntagPeer(peer, CMGR_TAG)
		},
	}

	l.p2p.Streams.Register(stream)
	stream.startStreaming()
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
	ok, err := l.p2p.ListenersP2P.Deregister(l.proto)
	if err != nil {
		return err
	}
	if ok {
		l.p2p.peerHost.RemoveStreamHandler(l.proto)
	}
	return nil
}
