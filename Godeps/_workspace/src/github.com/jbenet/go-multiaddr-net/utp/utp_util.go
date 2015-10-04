package utp

import (
	"errors"
	"net"
	"time"

	utp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/utp"
)

type Socket struct {
	s *utp.Socket

	Timeout time.Duration
}

type Dialer struct {
	s         *Socket
	Timeout   time.Duration
	LocalAddr net.Addr
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

func NewSocket(network string, laddr string) (*Socket, error) {
	switch network {
	case "utp", "utp4", "utp6":
		s, err := utp.NewSocket("udp"+network[3:], laddr)
		if err != nil {
			return nil, err
		}

		return &Socket{
			s: s,
		}, nil

	default:
		return nil, errors.New("unrecognized network: " + network)
	}
}

func (u *Socket) Close() error {
	return u.s.Close()
}

func (u *Socket) Accept() (net.Conn, error) {
	c, err := u.s.Accept()
	if err != nil {
		return nil, err
	}

	return &Conn{c}, nil
}

func (u *Socket) Addr() net.Addr {
	return MakeAddr(u.s.Addr())
}

func (d *Socket) Dial(rnet string, raddr string) (net.Conn, error) {
	c, err := d.s.DialTimeout(raddr, d.Timeout)
	if err != nil {
		return nil, err
	}

	return &Conn{c}, nil
}

func DialTimeout(rnet, raddr string, timeout time.Duration) (net.Conn, error) {
	c, err := utp.DialTimeout(raddr, timeout)
	if err != nil {
		return nil, err
	}

	return &Conn{c}, nil
}

func (d *Dialer) Dial(rnet string, raddr string) (net.Conn, error) {
	if d.LocalAddr != nil && d.s == nil {
		s, err := NewSocket(d.LocalAddr.Network(), d.LocalAddr.String())
		if err != nil {
			return nil, err
		}

		s.Timeout = d.Timeout
		d.s = s
	}

	var c net.Conn
	var err error
	if d.s != nil {
		// zero timeout is the same as calling s.Dial()
		c, err = d.s.Dial(rnet, raddr)
	} else {
		c, err = utp.DialTimeout(raddr, d.Timeout)
	}

	if err != nil {
		return nil, err
	}

	return &Conn{c}, nil
}
