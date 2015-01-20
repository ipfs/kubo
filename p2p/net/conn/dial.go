package conn

import (
	"fmt"
	"math/rand"
	"net"
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	reuseport "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"

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

	maconn, err := d.rawConnDial(ctx, raddr, remote)
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

// rawConnDial dials the underlying net.Conn + manet.Conns
func (d *Dialer) rawConnDial(ctx context.Context, raddr ma.Multiaddr, remote peer.ID) (manet.Conn, error) {

	// before doing anything, check we're going to be able to dial.
	// we may not support the given address.
	_, _, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(raddr.String(), "/ip4/0.0.0.0") {
		return nil, debugerror.Errorf("Attempted to connect to zero address: %s", raddr)
	}

	// get local addr to use.
	laddr := pickLocalAddr(d.LocalAddrs, raddr)

	log.Debugf("%s dialing %s -- %s --> %s", d.LocalPeer, remote, laddr, raddr)
	if laddr != nil {
		// dial using reuseport.Dialer, because we're probably reusing addrs.
		// this is optimistic, as the reuseDial may fail to bind the port.
		if nconn, err := d.reuseDial(laddr, raddr); err == nil {
			// if it worked, wrap the raw net.Conn with our manet.Conn
			log.Debugf("%s reuse worked! %s %s %s", d.LocalPeer, laddr, nconn.RemoteAddr(), nconn)
			return manet.WrapNetConn(nconn)
		} else {
			log.Debugf("%s port reuse failed: %s %s", d.LocalPeer, laddr, err)
		}
		// if not, we fall back to regular Dial without a local addr specified.
	}

	// no local addr, or failed to reuse. just dial straight with a new port.
	return d.Dialer.Dial(raddr)
}

func (d *Dialer) reuseDial(laddr, raddr ma.Multiaddr) (net.Conn, error) {
	// give reuse.Dialer the manet.Dialer's Dialer.
	// (wow, Dialer should've so been an interface...)
	rd := reuseport.Dialer{d.Dialer.Dialer}

	// get the local net.Addr manually
	var err error
	rd.D.LocalAddr, err = manet.ToNetAddr(laddr)
	if err != nil {
		return nil, err
	}

	// get the raddr dial args for rd.dial
	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	// rd.Dial gets us a net.Conn with SO_REUSEPORT and SO_REUSEADDR set.
	return rd.Dial(network, netraddr)
}

func pickLocalAddr(laddrs []ma.Multiaddr, raddr ma.Multiaddr) (laddr ma.Multiaddr) {
	if len(laddrs) < 1 {
		return nil
	}

	laddrs = manet.AddrMatch(raddr, laddrs)
	if len(laddrs) < 1 {
		return nil
	}

	// TODO pick with a good heuristic
	// we use a random one for now to prevent bad addresses from making nodes unreachable
	// with a random selection, multiple tries may work.
	return laddrs[rand.Intn(len(laddrs))]
}

// MultiaddrProtocolsMatch returns whether two multiaddrs match in protocol stacks.
func MultiaddrProtocolsMatch(a, b ma.Multiaddr) bool {
	ap := a.Protocols()
	bp := b.Protocols()

	if len(ap) != len(bp) {
		return false
	}

	for i, api := range ap {
		if api.Code != bp[i].Code {
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
