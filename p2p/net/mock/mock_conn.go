package mocknet

import (
	"container/list"
	"sync"

	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// conn represents one side's perspective of a
// live connection between two peers.
// it goes over a particular link.
type conn struct {
	local  peer.ID
	remote peer.ID

	localAddr  ma.Multiaddr
	remoteAddr ma.Multiaddr

	localPrivKey ic.PrivKey
	remotePubKey ic.PubKey

	net     *peernet
	link    *link
	rconn   *conn // counterpart
	streams list.List

	sync.RWMutex
}

func (c *conn) Close() error {
	for _, s := range c.allStreams() {
		s.Close()
	}
	c.net.removeConn(c)
	c.net.notifyAll(func(n inet.Notifiee) {
		n.Disconnected(c.net, c)
	})
	return nil
}

func (c *conn) addStream(s *stream) {
	c.Lock()
	s.conn = c
	c.streams.PushBack(s)
	c.Unlock()
}

func (c *conn) removeStream(s *stream) {
	c.Lock()
	defer c.Unlock()
	for e := c.streams.Front(); e != nil; e = e.Next() {
		if s == e.Value {
			c.streams.Remove(e)
			return
		}
	}
}

func (c *conn) allStreams() []inet.Stream {
	c.RLock()
	defer c.RUnlock()

	strs := make([]inet.Stream, 0, c.streams.Len())
	for e := c.streams.Front(); e != nil; e = e.Next() {
		s := e.Value.(*stream)
		strs = append(strs, s)
	}
	return strs
}

func (c *conn) remoteOpenedStream(s *stream) {
	c.addStream(s)
	c.net.handleNewStream(s)
	c.net.notifyAll(func(n inet.Notifiee) {
		n.OpenedStream(c.net, s)
	})
}

func (c *conn) openStream() *stream {
	sl, sr := c.link.newStreamPair()
	c.addStream(sl)
	c.net.notifyAll(func(n inet.Notifiee) {
		n.OpenedStream(c.net, sl)
	})
	c.rconn.remoteOpenedStream(sr)
	return sl
}

func (c *conn) NewStream() (inet.Stream, error) {
	log.Debugf("Conn.NewStreamWithProtocol: %s --> %s", c.local, c.remote)

	s := c.openStream()
	return s, nil
}

// LocalMultiaddr is the Multiaddr on this side
func (c *conn) LocalMultiaddr() ma.Multiaddr {
	return c.localAddr
}

// LocalPeer is the Peer on our side of the connection
func (c *conn) LocalPeer() peer.ID {
	return c.local
}

// LocalPrivateKey is the private key of the peer on our side.
func (c *conn) LocalPrivateKey() ic.PrivKey {
	return c.localPrivKey
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *conn) RemoteMultiaddr() ma.Multiaddr {
	return c.remoteAddr
}

// RemotePeer is the Peer on the remote side
func (c *conn) RemotePeer() peer.ID {
	return c.remote
}

// RemotePublicKey is the private key of the peer on our side.
func (c *conn) RemotePublicKey() ic.PubKey {
	return c.remotePubKey
}
