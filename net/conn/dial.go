package conn

import (
	"fmt"
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/peer"
)

// Dial connects to a particular peer, over a given network
// Example: d.Dial(ctx, "udp", peer)
func (d *Dialer) Dial(ctx context.Context, network string, remote peer.Peer) (Conn, error) {
	laddr := d.LocalPeer.NetAddress(network)
	if laddr == nil {
		return nil, fmt.Errorf("No local address for network %s", network)
	}

	raddr := remote.NetAddress(network)
	if raddr == nil {
		return nil, fmt.Errorf("No remote address for network %s", network)
	}

	if strings.HasPrefix(raddr.String(), "/ip4/0.0.0.0") {
		return nil, fmt.Errorf("Attempted to connect to zero address: %s", raddr)
	}

	remote.SetType(peer.Remote)
	remote, err := d.Peerstore.Add(remote)
	if err != nil {
		log.Errorf("Error putting peer into peerstore: %s", remote)
	}

	// TODO: try to get reusing addr/ports to work.
	// madialer := manet.Dialer{LocalAddr: laddr}
	madialer := manet.Dialer{}

	log.Infof("%s dialing %s %s", d.LocalPeer, remote, raddr)
	maconn, err := madialer.Dial(raddr)
	if err != nil {
		return nil, err
	}

	c, err := newSingleConn(ctx, d.LocalPeer, remote, maconn)
	if err != nil {
		return nil, err
	}

	return newSecureConn(ctx, c, d.Peerstore)
}
