package transport

import (
	"net"
	"sync"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	reuseport "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"
)

type TcpTransport struct {
	dlock   sync.Mutex
	dialers map[string]Dialer

	llock     sync.Mutex
	listeners map[string]Listener
}

func NewTCPTransport() *TcpTransport {
	return &TcpTransport{
		dialers:   make(map[string]Dialer),
		listeners: make(map[string]Listener),
	}
}

func (t *TcpTransport) Dialer(laddr ma.Multiaddr) (Dialer, error) {
	t.dlock.Lock()
	defer t.dlock.Unlock()
	s := laddr.String()
	d, found := t.dialers[s]
	if found {
		return d, nil
	}

	var base manet.Dialer
	tcpd, err := t.newTcpDialer(base, laddr)
	if err != nil {
		return nil, err
	}

	t.dialers[s] = tcpd
	return tcpd, nil
}

func (t *TcpTransport) Listener(laddr ma.Multiaddr) (Listener, error) {
	t.llock.Lock()
	defer t.llock.Unlock()
	s := laddr.String()
	l, found := t.listeners[s]
	if found {
		return l, nil
	}

	list, err := manetListen(laddr)
	if err != nil {
		return nil, err
	}

	tlist := &tcpListener{
		list:      list,
		transport: t,
	}

	t.listeners[s] = tlist
	return tlist, nil
}

func manetListen(addr ma.Multiaddr) (manet.Listener, error) {
	network, naddr, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	if ReuseportIsAvailable() {
		nl, err := reuseport.Listen(network, naddr)
		if err == nil {
			// hey, it worked!
			return manet.WrapNetListener(nl)
		}
		// reuseport is available, but we failed to listen. log debug, and retry normally.
		log.Debugf("reuseport available, but failed to listen: %s %s, %s", network, naddr, err)
	}

	// either reuseport not available, or it failed. try normally.
	return manet.Listen(addr)
}

func (t *TcpTransport) Matches(a ma.Multiaddr) bool {
	return IsTcpMultiaddr(a)
}

type tcpDialer struct {
	laddr ma.Multiaddr

	doReuse bool

	rd       reuseport.Dialer
	madialer manet.Dialer

	transport Transport
}

func (t *TcpTransport) newTcpDialer(base manet.Dialer, laddr ma.Multiaddr) (*tcpDialer, error) {
	// get the local net.Addr manually
	la, err := manet.ToNetAddr(laddr)
	if err != nil {
		return nil, err // something wrong with laddr.
	}

	if !ReuseportIsAvailable() {
		return &tcpDialer{
			doReuse:   false,
			laddr:     laddr,
			madialer:  base,
			transport: t,
		}, nil
	}

	return &tcpDialer{
		doReuse:   true,
		laddr:     laddr,
		rd:        reuseport.Dialer{D: net.Dialer{LocalAddr: la}},
		madialer:  base,
		transport: t,
	}, nil
}

func (d *tcpDialer) Dial(raddr ma.Multiaddr) (Conn, error) {
	var c manet.Conn
	var err error
	if d.doReuse {
		c, err = d.reuseDial(raddr)
	} else {
		c, err = d.madialer.Dial(raddr)
	}

	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      c,
		transport: d.transport,
	}, nil
}

func (d *tcpDialer) reuseDial(raddr ma.Multiaddr) (manet.Conn, error) {
	logdial := lgbl.Dial("conn", "", "", d.laddr, raddr)
	rpev := log.EventBegin(context.TODO(), "tptDialReusePort", logdial)

	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	con, err := d.rd.Dial(network, netraddr)
	if err == nil {
		logdial["reuseport"] = "success"
		rpev.Done()
		return manet.WrapNetConn(con)
	}

	if !ReuseErrShouldRetry(err) {
		logdial["reuseport"] = "failure"
		logdial["error"] = err
		rpev.Done()
		return nil, err
	}

	logdial["reuseport"] = "retry"
	logdial["error"] = err
	rpev.Done()

	return d.madialer.Dial(raddr)
}

func (d *tcpDialer) Matches(a ma.Multiaddr) bool {
	return IsTcpMultiaddr(a)
}

type tcpListener struct {
	list      manet.Listener
	transport Transport
}

func (d *tcpListener) Accept() (Conn, error) {
	c, err := d.list.Accept()
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      c,
		transport: d.transport,
	}, nil
}

func (d *tcpListener) Addr() net.Addr {
	return d.list.Addr()
}

func (t *tcpListener) Multiaddr() ma.Multiaddr {
	return t.list.Multiaddr()
}

func (t *tcpListener) NetListener() net.Listener {
	return t.list.NetListener()
}

func (d *tcpListener) Close() error {
	return d.list.Close()
}
