package transport

import (
	"net"
	"sync"

	ma "gx/ipfs/QmR3JkmZBKYXgNMNsNZawm914455Qof3PEopwuVSeXG7aV/go-multiaddr"
	utp "gx/ipfs/QmVs3wq4cN64TFCxANzgSHjGPrjMnRnwPrxU8bqc7YP42s/utp"
	manet "gx/ipfs/QmYtzQmUwPFGxjCXctJ8e6GXS8sYfoXy2pdeMbS5SFWqRi/go-multiaddr-net"
	mautp "gx/ipfs/QmYtzQmUwPFGxjCXctJ8e6GXS8sYfoXy2pdeMbS5SFWqRi/go-multiaddr-net/utp"
)

type UtpTransport struct {
	sockLock sync.Mutex
	sockets  map[string]*UtpSocket
}

func NewUtpTransport() *UtpTransport {
	return &UtpTransport{
		sockets: make(map[string]*UtpSocket),
	}
}

func (d *UtpTransport) Matches(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

type UtpSocket struct {
	s         *utp.Socket
	laddr     ma.Multiaddr
	transport Transport
}

func (t *UtpTransport) Listen(laddr ma.Multiaddr) (Listener, error) {
	t.sockLock.Lock()
	defer t.sockLock.Unlock()
	s, ok := t.sockets[laddr.String()]
	if ok {
		return s, nil
	}

	ns, err := t.newConn(laddr)
	if err != nil {
		return nil, err
	}

	t.sockets[laddr.String()] = ns
	return ns, nil
}

func (t *UtpTransport) Dialer(laddr ma.Multiaddr, opts ...DialOpt) (Dialer, error) {
	t.sockLock.Lock()
	defer t.sockLock.Unlock()
	s, ok := t.sockets[laddr.String()]
	if ok {
		return s, nil
	}

	ns, err := t.newConn(laddr, opts...)
	if err != nil {
		return nil, err
	}

	t.sockets[laddr.String()] = ns
	return ns, nil
}

func (t *UtpTransport) newConn(addr ma.Multiaddr, opts ...DialOpt) (*UtpSocket, error) {
	network, netaddr, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	s, err := utp.NewSocket("udp"+network[3:], netaddr)
	if err != nil {
		return nil, err
	}

	laddr, err := manet.FromNetAddr(mautp.MakeAddr(s.LocalAddr()))
	if err != nil {
		return nil, err
	}

	return &UtpSocket{
		s:         s,
		laddr:     laddr,
		transport: t,
	}, nil
}

func (s *UtpSocket) Dial(raddr ma.Multiaddr) (Conn, error) {
	_, addr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	con, err := s.s.Dial(addr)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(&mautp.Conn{Conn: con})
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      mnc,
		transport: s.transport,
	}, nil
}

func (s *UtpSocket) Accept() (Conn, error) {
	c, err := s.s.Accept()
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(&mautp.Conn{Conn: c})
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      mnc,
		transport: s.transport,
	}, nil
}

func (s *UtpSocket) Matches(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

func (t *UtpSocket) Close() error {
	return t.s.Close()
}

func (t *UtpSocket) Addr() net.Addr {
	return t.s.Addr()
}

func (t *UtpSocket) Multiaddr() ma.Multiaddr {
	return t.laddr
}

var _ Transport = (*UtpTransport)(nil)
