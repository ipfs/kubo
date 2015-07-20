package mocknet

import (
	//	"fmt"
	"io"
	"sync"
	"time"

	inet "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

// link implements mocknet.Link
// and, for simplicity, inet.Conn
type link struct {
	mock        *mocknet
	nets        []*peernet
	opts        LinkOptions
	ratelimiter *ratelimiter
	// this could have addresses on both sides.

	sync.RWMutex
}

func newLink(mn *mocknet, opts LinkOptions) *link {
	l := &link{mock: mn,
		opts:        opts,
		ratelimiter: NewRatelimiter(opts.Bandwidth)}
	return l
}

func (l *link) newConnPair(dialer *peernet) (*conn, *conn) {
	l.RLock()
	defer l.RUnlock()

	mkconn := func(ln, rn *peernet) *conn {
		c := &conn{net: ln, link: l}
		c.local = ln.peer
		c.remote = rn.peer

		c.localAddr = ln.ps.Addrs(ln.peer)[0]
		c.remoteAddr = rn.ps.Addrs(rn.peer)[0]

		c.localPrivKey = ln.ps.PrivKey(ln.peer)
		c.remotePubKey = rn.ps.PubKey(rn.peer)

		return c
	}

	c1 := mkconn(l.nets[0], l.nets[1])
	c2 := mkconn(l.nets[1], l.nets[0])
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

	s1 := NewStream(w2, r1)
	s2 := NewStream(w1, r2)
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
	l.ratelimiter.UpdateBandwidth(l.opts.Bandwidth)
}

func (l *link) Options() LinkOptions {
	return l.opts
}

func (l *link) GetLatency() time.Duration {
	return l.opts.Latency
}

func (l *link) RateLimit(dataSize int) time.Duration {
	return l.ratelimiter.Limit(dataSize)
}
