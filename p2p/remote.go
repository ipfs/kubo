package p2p

import (
	"context"
	"fmt"

	net "github.com/libp2p/go-libp2p-core/network"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var maPrefix = "/" + ma.ProtocolWithCode(ma.P_IPFS).Name + "/"

// remoteListener accepts libp2p streams and proxies them to a manet host
type remoteListener struct {
	p2p *P2P

	// Application proto identifier.
	proto protocol.ID

	// Address to proxy the incoming connections to
	addr ma.Multiaddr

	// reportRemote if set to true makes the handler send '<base58 remote peerid>\n'
	// to target before any data is forwarded
	reportRemote bool
}

// ForwardRemote creates new p2p listener
func (p2p *P2P) ForwardRemote(ctx context.Context, proto protocol.ID, addr ma.Multiaddr, reportRemote bool) (Listener, error) {
	listener := &remoteListener{
		p2p: p2p,

		proto: proto,
		addr:  addr,

		reportRemote: reportRemote,
	}

	if err := p2p.ListenersP2P.Register(listener); err != nil {
		return nil, err
	}

	return listener, nil
}

func (l *remoteListener) handleStream(remote net.Stream) {
	local, err := manet.Dial(l.addr)
	if err != nil {
		_ = remote.Reset()
		return
	}

	peer := remote.Conn().RemotePeer()

	if l.reportRemote {
		if _, err := fmt.Fprintf(local, "%s\n", peer.Pretty()); err != nil {
			_ = remote.Reset()
			return
		}
	}

	peerMa, err := ma.NewMultiaddr(maPrefix + peer.Pretty())
	if err != nil {
		_ = remote.Reset()
		return
	}

	stream := &Stream{
		Protocol: l.proto,

		OriginAddr: peerMa,
		TargetAddr: l.addr,
		peer:       peer,

		Local:  local,
		Remote: remote,

		Registry: l.p2p.Streams,
	}

	l.p2p.Streams.Register(stream)
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

func (l *remoteListener) close() {}

func (l *remoteListener) key() string {
	return string(l.proto)
}
