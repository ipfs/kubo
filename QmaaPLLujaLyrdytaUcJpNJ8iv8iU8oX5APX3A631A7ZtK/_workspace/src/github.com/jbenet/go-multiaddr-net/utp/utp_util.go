package utp

import (
	"errors"
	"net"
	"time"

	utp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/utp"
)

type Listener struct {
	*utp.Socket
}

type Conn struct {
	net.Conn
}

type Addr struct {
	net   string
	child net.Addr
}

func (ca *Addr) Network() string {
	return ca.net
}

func (ca *Addr) String() string {
	return ca.child.String()
}

func (ca *Addr) Child() net.Addr {
	return ca.child
}

func MakeAddr(a net.Addr) net.Addr {
	return &Addr{
		net:   "utp",
		child: a,
	}
}

func ResolveAddr(network string, host string) (net.Addr, error) {
	a, err := net.ResolveUDPAddr("udp"+network[3:], host)
	if err != nil {
		return nil, err
	}

	return MakeAddr(a), nil
}

func (u *Conn) LocalAddr() net.Addr {
	return MakeAddr(u.Conn.LocalAddr())
}

func (u *Conn) RemoteAddr() net.Addr {
	return MakeAddr(u.Conn.RemoteAddr())
}

func Listen(network string, laddr string) (net.Listener, error) {
	switch network {
	case "utp", "utp4", "utp6":
		s, err := utp.NewSocket("udp"+network[3:], laddr)
		if err != nil {
			return nil, err
		}

		return &Listener{s}, nil

	default:
		return nil, errors.New("unrecognized network: " + network)
	}
}

func (u *Listener) Accept() (net.Conn, error) {
	c, err := u.Socket.Accept()
	if err != nil {
		return nil, err
	}

	return &Conn{c}, nil
}

func (u *Listener) Addr() net.Addr {
	return MakeAddr(u.Socket.Addr())
}

type Dialer struct {
	Timeout   time.Duration
	LocalAddr net.Addr
}

func (d *Dialer) Dial(rnet string, raddr string) (net.Conn, error) {
	if d.LocalAddr != nil {
		s, err := utp.NewSocket(d.LocalAddr.Network(), d.LocalAddr.String())
		if err != nil {
			return nil, err
		}

		// zero timeout is the same as calling s.Dial()
		return s.DialTimeout(raddr, d.Timeout)
	}

	return utp.DialTimeout(raddr, d.Timeout)
}
