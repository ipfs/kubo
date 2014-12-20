package mocknet

import (
	"io"
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
)

// link implements mocknet.Link
// and, for simplicity, inet.Conn
type link struct {
	mock *mocknet
	nets []*peernet
	opts LinkOptions

	sync.RWMutex
}

func newLink(mn *mocknet, opts LinkOptions) *link {
	return &link{mock: mn, opts: opts}
}

func (l *link) newConnPair(dialer *peernet) (*conn, *conn) {
	l.RLock()
	defer l.RUnlock()

	mkconn := func(n *peernet, rid peer.ID) *conn {
		c := &conn{net: n, link: l}
		c.local = n.peer
		c.remote = rid
		return c
	}

	c1 := mkconn(l.nets[0], l.nets[1].peer)
	c2 := mkconn(l.nets[1], l.nets[0].peer)
	c1.rconn = c2
	c2.rconn = c1

	if dialer == c1.net {
		return c1, c2
	}
	return c2, c1
}

func (l *link) newStreamPair() (*stream, *stream) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	s1 := &stream{Reader: r1, Writer: w2}
	s2 := &stream{Reader: r2, Writer: w1}
	return s1, s2
}

func (l *link) Networks() []inet.Network {
	l.RLock()
	defer l.RUnlock()

	cp := make([]inet.Network, len(l.nets))
	for i, n := range l.nets {
		cp[i] = n
	}
	return cp
}

func (l *link) Peers() []peer.ID {
	l.RLock()
	defer l.RUnlock()

	cp := make([]peer.ID, len(l.nets))
	for i, n := range l.nets {
		cp[i] = n.peer
	}
	return cp
}

func (l *link) SetOptions(o LinkOptions) {
	l.opts = o
}

func (l *link) Options() LinkOptions {
	return l.opts
}
