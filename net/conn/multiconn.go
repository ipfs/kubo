package conn

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
)

// MultiConnMap is for shorthand
type MultiConnMap map[u.Key]*MultiConn

// Duplex is a simple duplex channel
type Duplex struct {
	In  chan []byte
	Out chan []byte
}

// MultiConn represents a single connection to another Peer (IPFS Node).
type MultiConn struct {

	// connections, mapped by a string, which uniquely identifies the connection.
	// this string is:  /addr1/peer1/addr2/peer2 (peers ordered lexicographically)
	conns map[string]Conn

	local  peer.Peer
	remote peer.Peer

	// fan-in/fan-out
	duplex Duplex

	// for adding/removing connections concurrently
	sync.RWMutex
	ctxc.ContextCloser
}

// NewMultiConn constructs a new connection
func NewMultiConn(ctx context.Context, local, remote peer.Peer, conns []Conn) (*MultiConn, error) {

	c := &MultiConn{
		local:  local,
		remote: remote,
		conns:  map[string]Conn{},
		duplex: Duplex{
			In:  make(chan []byte, 10),
			Out: make(chan []byte, 10),
		},
	}

	// must happen before Adds / fanOut
	c.ContextCloser = ctxc.NewContextCloser(ctx, c.close)

	if conns != nil && len(conns) > 0 {
		c.Add(conns...)
	}

	c.Children().Add(1)
	go c.fanOut()
	return c, nil
}

// Add adds given Conn instances to multiconn.
func (c *MultiConn) Add(conns ...Conn) {
	c.Lock()
	defer c.Unlock()

	for _, c2 := range conns {
		log.Debugf("MultiConn: adding %s", c2)
		if c.LocalPeer() != c2.LocalPeer() || c.RemotePeer() != c2.RemotePeer() {
			log.Error(c2)
			c.Unlock() // ok to unlock (to log). panicing.
			log.Error(c)
			// log.Errorf("c.LocalPeer: %s %p", c.LocalPeer(), c.LocalPeer())
			// log.Errorf("c2.LocalPeer: %s %p", c2.LocalPeer(), c2.LocalPeer())
			// log.Errorf("c.RemotePeer: %s %p", c.RemotePeer(), c.RemotePeer())
			// log.Errorf("c2.RemotePeer: %s %p", c2.RemotePeer(), c2.RemotePeer())
			c.Lock() // gotta relock to avoid lock panic from deferring.
			panic("connection addresses mismatch")
		}

		c.conns[c2.ID()] = c2
		c.Children().Add(1)
		c2.Children().Add(1) // yep, on the child too.
		go c.fanInSingle(c2)
		log.Debugf("MultiConn: added %s", c2)
	}
}

// Remove removes given Conn instances from multiconn.
func (c *MultiConn) Remove(conns ...Conn) {

	// first remove them to avoid sending any more messages through it.
	{
		c.Lock()
		for _, c1 := range conns {
			c2, found := c.conns[c1.ID()]
			if !found {
				panic("Conn not in MultiConn")
			}
			if c1 != c2 {
				panic("different Conn objects for same id.")
			}

			delete(c.conns, c2.ID())
		}
		c.Unlock()
	}

	// close all in parallel, but wait for all to be done closing.
	CloseConns(conns...)
}

// CloseConns closes multiple connections in parallel, and waits for all
// to finish closing.
func CloseConns(conns ...Conn) {
	var wg sync.WaitGroup
	for _, child := range conns {

		select {
		case <-child.Closed(): // if already closed, continue
			continue
		default:
		}

		wg.Add(1)
		go func(child Conn) {
			child.Close()
			wg.Done()
		}(child)
	}
	wg.Wait()
}

// fanOut is the multiplexor out -- it sends outgoing messages over the
// underlying single connections.
func (c *MultiConn) fanOut() {
	defer c.Children().Done()

	i := 0
	for {
		select {
		case <-c.Closing():
			return

		// send data out through our "best connection"
		case m, more := <-c.duplex.Out:
			if !more {
				log.Debugf("%s out channel closed", c)
				return
			}
			sc := c.BestConn()
			if sc == nil {
				// maybe this should be a logged error, not a panic.
				panic("sending out multiconn without any live connection")
			}

			i++
			log.Debugf("%s sending (%d)", sc, i)
			sc.Out() <- m
		}
	}
}

// fanInSingle is a multiplexor in -- it receives incoming messages over the
// underlying single connections.
func (c *MultiConn) fanInSingle(child Conn) {
	// cleanup all data associated with this child Connection.
	defer func() {
		log.Debugf("closing: %s", child)

		// in case it still is in the map, remove it.
		c.Lock()
		delete(c.conns, child.ID())
		connLen := len(c.conns)
		c.Unlock()

		c.Children().Done()
		child.Children().Done()

		if connLen == 0 {
			c.Close() // close self if all underlying children are gone?
		}
	}()

	i := 0
	for {
		select {
		case <-c.Closing(): // multiconn closing
			return

		case <-child.Closing(): // child closing
			return

		case m, more := <-child.In(): // receiving data
			if !more {
				log.Debugf("%s in channel closed", child)
				err := c.GetError()
				if err != nil {
					log.Errorf("Found error on connection: %s", err)
				}
				return // closed
			}
			i++
			log.Debugf("%s received (%d)", child, i)
			c.duplex.In <- m
		}
	}
}

// close is the internal close function, called by ContextCloser.Close
func (c *MultiConn) close() error {
	log.Debugf("%s closing Conn with %s", c.local, c.remote)

	// get connections
	c.RLock()
	conns := make([]Conn, 0, len(c.conns))
	for _, c := range c.conns {
		conns = append(conns, c)
	}
	c.RUnlock()

	// close underlying connections
	CloseConns(conns...)
	return nil
}

// BestConn is the best connection in this MultiConn
func (c *MultiConn) BestConn() Conn {
	c.RLock()
	defer c.RUnlock()

	var id1 string
	var c1 Conn
	for id2, c2 := range c.conns {
		if id1 == "" || id1 < id2 {
			id1 = id2
			c1 = c2
		}
	}
	return c1
}

// ID is an identifier unique to this connection.
// In MultiConn, this is all the children IDs XORed together.
func (c *MultiConn) ID() string {
	c.RLock()
	defer c.RUnlock()

	ids := []byte(nil)
	for i := range c.conns {
		if ids == nil {
			ids = []byte(i)
		} else {
			ids = u.XOR(ids, []byte(i))
		}
	}

	return string(ids)
}

func (c *MultiConn) getConns() []Conn {
	c.RLock()
	defer c.RUnlock()
	var conns []Conn
	for _, c := range c.conns {
		conns = append(conns, c)
	}
	return conns
}

func (c *MultiConn) String() string {
	return String(c, "MultiConn")
}

// LocalMultiaddr is the Multiaddr on this side
func (c *MultiConn) LocalMultiaddr() ma.Multiaddr {
	return c.BestConn().LocalMultiaddr()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *MultiConn) RemoteMultiaddr() ma.Multiaddr {
	return c.BestConn().RemoteMultiaddr()
}

// LocalPeer is the Peer on this side
func (c *MultiConn) LocalPeer() peer.Peer {
	return c.local
}

// RemotePeer is the Peer on the remote side
func (c *MultiConn) RemotePeer() peer.Peer {
	return c.remote
}

// In returns a readable message channel
func (c *MultiConn) In() <-chan []byte {
	return c.duplex.In
}

// Out returns a writable message channel
func (c *MultiConn) Out() chan<- []byte {
	return c.duplex.Out
}

// BytesRead returns the total number of bytes received from the peer
func (c *MultiConn) BytesRead() int {
	c.RLock()
	defer c.RUnlock()

	sum := 0
	for _, conn := range c.conns {
		sum += conn.BytesRead()
	}
	return sum
}

// BytesWritten returns the total number of bytes sent to the peer
func (c *MultiConn) BytesWritten() int {
	c.RLock()
	defer c.RUnlock()

	sum := 0
	for _, conn := range c.conns {
		sum += conn.BytesWritten()
	}
	return sum
}

func (c *MultiConn) GetError() error {
	c.RLock()
	defer c.RUnlock()
	for _, sub := range c.conns {
		err := sub.GetError()
		if err != nil {
			return err
		}
	}
	return nil
}
