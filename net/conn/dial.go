package conn

import (
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/peer"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

// Dial connects to a particular peer, over a given network
// Example: d.Dial(ctx, "udp", peer)
func (d *Dialer) Dial(ctx context.Context, network string, remote peer.Peer) (Conn, error) {
	raddr := remote.NetAddress(network)
	if raddr == nil {
		return nil, debugerror.Errorf("No remote address for network %s", network)
	}
	return d.DialAddr(ctx, raddr, remote)
}

// DialAddr connects to a peer over a particular address
// Ensures raddr is part of peer.Addresses()
// Example: d.DialAddr(ctx, peer.Addresses()[0], peer)
func (d *Dialer) DialAddr(ctx context.Context, raddr ma.Multiaddr, remote peer.Peer) (Conn, error) {

	found := false
	for _, addr := range remote.Addresses() {
		if addr.Equal(raddr) {
			found = true
		}
	}
	if !found {
		return nil, debugerror.Errorf("address %s is not in peer %s", raddr, remote)
	}

	network, _, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	laddr := d.LocalPeer.NetAddress(network)
	if laddr == nil {
		return nil, debugerror.Errorf("No local address for network %s", network)
	}

	if strings.HasPrefix(raddr.String(), "/ip4/0.0.0.0") {
		return nil, debugerror.Errorf("Attempted to connect to zero address: %s", raddr)
	}

	remote.SetType(peer.Remote)
	remote, err = d.Peerstore.Add(remote)
	if err != nil {
		log.Errorf("Error putting peer into peerstore: %s", remote)
	}

	// TODO: try to get reusing addr/ports to work.
	// madialer := manet.Dialer{LocalAddr: laddr}
	madialer := manet.Dialer{}

	log.Debugf("%s dialing %s %s", d.LocalPeer, remote, raddr)
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
