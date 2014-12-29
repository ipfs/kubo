package conn

import (
	"fmt"
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

// String returns the string rep of d.
func (d *Dialer) String() string {
	return fmt.Sprintf("<Dialer %s %s ...>", d.LocalPeer, d.LocalAddrs[0])
}

// Dial connects to a peer over a particular address
// Ensures raddr is part of peer.Addresses()
// Example: d.DialAddr(ctx, peer.Addresses()[0], peer)
func (d *Dialer) Dial(ctx context.Context, raddr ma.Multiaddr, remote peer.ID) (Conn, error) {

	network, _, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(raddr.String(), "/ip4/0.0.0.0") {
		return nil, debugerror.Errorf("Attempted to connect to zero address: %s", raddr)
	}

	var laddr ma.Multiaddr
	if len(d.LocalAddrs) > 0 {
		// laddr := MultiaddrNetMatch(raddr, d.LocalAddrs)
		laddr = NetAddress(network, d.LocalAddrs)
		if laddr == nil {
			return nil, debugerror.Errorf("No local address for network %s", network)
		}
	}

	// TODO: try to get reusing addr/ports to work.
	// madialer := manet.Dialer{LocalAddr: laddr}
	madialer := manet.Dialer{}

	log.Debugf("%s dialing %s %s", d.LocalPeer, remote, raddr)
	maconn, err := madialer.Dial(raddr)
	if err != nil {
		return nil, err
	}

	var connOut Conn
	var errOut error
	done := make(chan struct{})

	// do it async to ensure we respect don contexteone
	go func() {
		defer func() { done <- struct{}{} }()

		c, err := newSingleConn(ctx, d.LocalPeer, remote, maconn)
		if err != nil {
			errOut = err
			return
		}

		if d.PrivateKey == nil {
			log.Warning("dialer %s dialing INSECURELY %s at %s!", d, remote, raddr)
			connOut = c
			return
		}
		c2, err := newSecureConn(ctx, d.PrivateKey, c)
		if err != nil {
			errOut = err
			c.Close()
			return
		}

		connOut = c2
	}()

	select {
	case <-ctx.Done():
		maconn.Close()
		return nil, ctx.Err()
	case <-done:
		// whew, finished.
	}

	return connOut, errOut
}

// MultiaddrProtocolsMatch returns whether two multiaddrs match in protocol stacks.
func MultiaddrProtocolsMatch(a, b ma.Multiaddr) bool {
	ap := a.Protocols()
	bp := b.Protocols()

	if len(ap) != len(bp) {
		return false
	}

	for i, api := range ap {
		if api != bp[i] {
			return false
		}
	}

	return true
}

// MultiaddrNetMatch returns the first Multiaddr found to match  network.
func MultiaddrNetMatch(tgt ma.Multiaddr, srcs []ma.Multiaddr) ma.Multiaddr {
	for _, a := range srcs {
		if MultiaddrProtocolsMatch(tgt, a) {
			return a
		}
	}
	return nil
}

// NetAddress returns the first Multiaddr found for a given network.
func NetAddress(n string, addrs []ma.Multiaddr) ma.Multiaddr {
	for _, a := range addrs {
		for _, p := range a.Protocols() {
			if p.Name == n {
				return a
			}
		}
	}
	return nil
}
