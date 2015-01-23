package conn

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"syscall"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	reuseport "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"

	addrutil "github.com/jbenet/go-ipfs/p2p/net/swarm/addr"
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
	if _, _, err := manet.DialArgs(raddr); err != nil {
		return nil, err
	}

	if strings.HasPrefix(raddr.String(), "/ip4/0.0.0.0") {
		return nil, debugerror.Errorf("Attempted to connect to zero address: %s", raddr)
	}

	// get local addr to use.
	laddr := pickLocalAddr(d.LocalAddrs, raddr)
	log.Debugf("%s dialing %s -- %s --> %s", d.LocalPeer, remote, laddr, raddr)

	if laddr != nil && reuseport.Available() {
		// dial using reuseport.Dialer, because we're probably reusing addrs.
		// this is optimistic, as the reuseDial may fail to bind the port.
		if nconn, retry, reuseErr := d.reuseDial(laddr, raddr); reuseErr == nil {
			// if it worked, wrap the raw net.Conn with our manet.Conn
			log.Debugf("%s reuse worked! %s %s %s", d.LocalPeer, laddr, nconn.RemoteAddr(), nconn)
			return manet.WrapNetConn(nconn)
		} else if !retry {
			// reuseDial is sure this is a legitimate dial failure, not a reuseport failure.
			return nil, reuseErr
		} else {
			// this is a failure to reuse port. log it.
			log.Debugf("%s port reuse failed: %s --> %s -- %s", d.LocalPeer, laddr, raddr, reuseErr)
		}
	}

	// no local addr, or reuseport failed. just dial straight with a new port.
	return d.Dialer.Dial(raddr)
}

func (d *Dialer) reuseDial(laddr, raddr ma.Multiaddr) (conn net.Conn, retry bool, err error) {
	if laddr == nil {
		// if we're given no local address no sense in using reuseport to dial, dial out as usual.
		return nil, true, reuseport.ErrReuseFailed
	}

	// half the timeout so we can retry regularly if this fails.
	d.Dialer.Dialer.Timeout = (d.Dialer.Dialer.Timeout / 2)

	// give reuse.Dialer the manet.Dialer's Dialer.
	// (wow, Dialer should've so been an interface...)
	rd := reuseport.Dialer{d.Dialer.Dialer}

	// get the local net.Addr manually
	rd.D.LocalAddr, err = manet.ToNetAddr(laddr)
	if err != nil {
		return nil, true, err // something wrong with laddr. retry without.
	}

	// get the raddr dial args for rd.dial
	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, true, err // something wrong with laddr. retry without.
	}

	// rd.Dial gets us a net.Conn with SO_REUSEPORT and SO_REUSEADDR set.
	conn, err = rd.Dial(network, netraddr)
	return conn, reuseErrShouldRetry(err), err // hey! it worked!
}

// reuseErrShouldRetry diagnoses whether to retry after a reuse error.
// if we failed to bind, we should retry. if bind worked and this is a
// real dial error (remote end didnt answer) then we should not retry.
func reuseErrShouldRetry(err error) bool {
	if err == nil {
		return false // hey, it worked! no need to retry.
	}

	// if it's a network timeout error, it's a legitimate failure.
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return true
	}

	errno, ok := err.(syscall.Errno)
	if !ok { // not an errno? who knows what this is. retry.
		return true
	}

	switch errno {
	case syscall.EADDRINUSE, syscall.EADDRNOTAVAIL:
		return true // failure to bind. retry.
	case syscall.ECONNREFUSED:
		return false // real dial error
	default:
		return true // optimistically default to retry.
	}
}

func pickLocalAddr(laddrs []ma.Multiaddr, raddr ma.Multiaddr) (laddr ma.Multiaddr) {
	if len(laddrs) < 1 {
		return nil
	}

	// make sure that we ONLY use local addrs that match the remote addr.
	laddrs = manet.AddrMatch(raddr, laddrs)
	if len(laddrs) < 1 {
		return nil
	}

	// make sure that we ONLY use local addrs that CAN dial the remote addr.
	// filter out all the local addrs that aren't capable
	raddrIPLayer := ma.Split(raddr)[0]
	raddrIsLoopback := manet.IsIPLoopback(raddrIPLayer)
	raddrIsLinkLocal := manet.IsIP6LinkLocal(raddrIPLayer)
	laddrs = addrutil.FilterAddrs(laddrs, func(a ma.Multiaddr) bool {
		laddrIPLayer := ma.Split(a)[0]
		laddrIsLoopback := manet.IsIPLoopback(laddrIPLayer)
		laddrIsLinkLocal := manet.IsIP6LinkLocal(laddrIPLayer)
		if laddrIsLoopback { // our loopback addrs can only dial loopbacks.
			return raddrIsLoopback
		}
		if laddrIsLinkLocal {
			return raddrIsLinkLocal // out linklocal addrs can only dial link locals.
		}
		return true
	})

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
