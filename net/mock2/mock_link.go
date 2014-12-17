package mocknet

import (
	"fmt"
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

func newLink(mn *mocknet) *link {
	return &link{mock: mn, opts: mn.linkDefaults}
}

func (l *link) newConnPair() (*conn, *conn) {
	l.RLock()
	defer l.RUnlock()

	mkconn := func(n *peernet, rid peer.ID) *conn {
		c := &conn{net: n, link: l}
		c.local = n.peer

		r, err := n.ps.FindOrCreate(rid)
		if err != nil {
			panic(fmt.Errorf("error creating peer: %s", err))
		}
		c.remote = r
		return c
	}

	c1 := mkconn(l.nets[0], l.nets[1].peer.ID())
	c2 := mkconn(l.nets[1], l.nets[0].peer.ID())
	c1.rconn = c2
	c2.rconn = c1
	return c1, c2
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

func (l *link) Peers() []peer.Peer {
	l.RLock()
	defer l.RUnlock()

	cp := make([]peer.Peer, len(l.nets))
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
