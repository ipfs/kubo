package conn

import (
	"errors"
	"net"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	mautp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net/utp"
	reuseport "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"

	"golang.org/x/net/context"
)

type Transport interface {
	manet.Listener
	ProtoDialer
}

type ProtoDialer interface {
	Dial(raddr ma.Multiaddr) (manet.Conn, error)
	Matches(ma.Multiaddr) bool
}

type TcpTransport struct {
	list  manet.Listener
	laddr ma.Multiaddr

	doReuse bool

	rd       reuseport.Dialer
	madialer manet.Dialer
}

var _ Transport = (*TcpTransport)(nil)

func NewTcpReuseTransport(base manet.Dialer, laddr ma.Multiaddr) (*TcpTransport, error) {
	rd := reuseport.Dialer{base.Dialer}

	// get the local net.Addr manually
	la, err := manet.ToNetAddr(laddr)
	if err != nil {
		return nil, err // something wrong with laddr.
	}
	rd.D.LocalAddr = la

	list, err := manet.Listen(laddr)
	if err != nil {
		return nil, err
	}

	return &TcpTransport{
		doReuse:  true,
		list:     list,
		laddr:    laddr,
		rd:       rd,
		madialer: base,
	}, nil
}

// NewTcpTransport creates a TcpTransport that does not use SO_REUSEPORT
func NewTcpTransport(base manet.Dialer, laddr ma.Multiaddr) (*TcpTransport, error) {
	list, err := manet.Listen(laddr)
	if err != nil {
		return nil, err
	}

	return &TcpTransport{
		list:     list,
		laddr:    laddr,
		madialer: base,
	}, nil

}

func (d *TcpTransport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	if d.doReuse {
		return d.reuseDial(raddr)
	}

	return d.madialer.Dial(raddr)
}

func (d *TcpTransport) reuseDial(raddr ma.Multiaddr) (manet.Conn, error) {
	logdial := lgbl.Dial("conn", "", "", d.laddr, raddr)
	rpev := log.EventBegin(context.TODO(), "tptDialReusePort", logdial)

	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	conn, err := d.rd.Dial(network, netraddr)
	if err == nil {
		logdial["reuseport"] = "success"
		rpev.Done()
		return manet.WrapNetConn(conn)
	}

	if !reuseErrShouldRetry(err) {
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

func (d *TcpTransport) Matches(a ma.Multiaddr) bool {
	return IsTcpMultiaddr(a)
}

func (d *TcpTransport) Accept() (manet.Conn, error) {
	c, err := d.list.Accept()
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (d *TcpTransport) Addr() net.Addr {
	return d.list.Addr()
}

func (t *TcpTransport) Multiaddr() ma.Multiaddr {
	return t.list.Multiaddr()
}

func (t *TcpTransport) NetListener() net.Listener {
	return t.list.NetListener()
}

func (d *TcpTransport) Close() error {
	return d.list.Close()
}

func IsTcpMultiaddr(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 2 && (p[0].Name == "ip4" || p[0].Name == "ip6") && p[1].Name == "tcp"
}

func IsUtpMultiaddr(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

type UtpTransport struct {
	s     *mautp.Socket
	laddr ma.Multiaddr
}

func NewUtpTransport(laddr ma.Multiaddr) (*UtpTransport, error) {
	network, addr, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}

	us, err := mautp.NewSocket(network, addr)
	if err != nil {
		return nil, err
	}

	mmm, err := manet.FromNetAddr(us.Addr())
	if err != nil {
		return nil, err
	}

	return &UtpTransport{
		s:     us,
		laddr: mmm,
	}, nil
}

func (d *UtpTransport) Matches(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

func (d *UtpTransport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	c, err := d.s.Dial(network, netraddr)
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (d *UtpTransport) Accept() (manet.Conn, error) {
	c, err := d.s.Accept()
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (t *UtpTransport) Close() error {
	return t.s.Close()
}

func (t *UtpTransport) Addr() net.Addr {
	return t.s.Addr()
}

func (t *UtpTransport) Multiaddr() ma.Multiaddr {
	return t.laddr
}

func (t *UtpTransport) NetListener() net.Listener {
	return t.s
}

type BasicMaDialer struct {
	Dialer manet.Dialer
}

func (d *BasicMaDialer) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	return d.Dialer.Dial(raddr)
}

func (d *BasicMaDialer) Matches(a ma.Multiaddr) bool {
	return true
}

var ErrNoSpecialTransport = errors.New("given multiaddr has no supported special transport")

func MakeTransport(laddr ma.Multiaddr, tout time.Duration) (Transport, error) {
	switch {
	case IsTcpMultiaddr(laddr):
		if !reuseportIsAvailable() {
			return nil, ErrNoSpecialTransport
		}
		dialer := manet.Dialer{Dialer: net.Dialer{Timeout: tout}}
		return NewTcpReuseTransport(dialer, laddr)

	case IsUtpMultiaddr(laddr):
		return NewUtpTransport(laddr)

	default:
		return nil, ErrNoSpecialTransport
	}
}
