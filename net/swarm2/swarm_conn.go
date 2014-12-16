package swarm

import (
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ps "github.com/jbenet/go-peerstream"
)

// a Conn is a simple wrapper around a ps.Conn that also exposes
// some of the methods from the underlying conn.Conn.
// There's **five** "layers" to each connection:
//  * 0. the net.Conn - underlying net.Conn (TCP/UDP/UTP/etc)
//  * 1. the manet.Conn - provides multiaddr friendly Conn
//  * 2. the conn.Conn - provides Peer friendly Conn (inc Secure channel)
//  * 3. the peerstream.Conn - provides peerstream / spdysptream happiness
//  * 4. the Conn - abstracts everyting out, exposing only key parts of underlying layers
// (I know, this is kinda crazy. it's more historical than a good design. though the
// layers do build up pieces of functionality. and they're all just io.RW :) )
type Conn ps.Conn

func (c *Conn) StreamConn() *ps.Conn {
	return (*ps.Conn)(c)
}

func (c *Conn) RawConn() conn.Conn {
	// righly panic if these things aren't true. it is an expected
	// invariant that these Conns are all of the typewe expect:
	// 		ps.Conn wrapping a conn.Conn
	// if we get something else it is programmer error.
	return (*ps.Conn)(c).NetConn().(conn.Conn)
}

// LocalMultiaddr is the Multiaddr on this side
func (c *Conn) LocalMultiaddr() ma.Multiaddr {
	return c.RawConn().LocalMultiaddr()
}

// LocalPeer is the Peer on our side of the connection
func (c *Conn) LocalPeer() peer.Peer {
	return c.RawConn().LocalPeer()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *Conn) RemoteMultiaddr() ma.Multiaddr {
	return c.RawConn().RemoteMultiaddr()
}

// RemotePeer is the Peer on the remote side
func (c *Conn) RemotePeer() peer.Peer {
	return c.RawConn().RemotePeer()
}

// NewStream returns a new Stream from this connection
func (c *Conn) NewStream() (*Stream, error) {
	s, err := c.StreamConn().NewStream()
	return wrapStream(s), err
}

func (c *Conn) Close() error {
	return c.StreamConn().Close()
}

func wrapConn(psc *ps.Conn) (*Conn, error) {
	// grab the underlying connection.
	if _, ok := psc.NetConn().(conn.Conn); !ok {
		// this should never happen. if we see it ocurring it means that we added
		// a Listener to the ps.Swarm that is NOT one of our net/conn.Listener.
		return nil, fmt.Errorf("swarm connHandler: invalid conn (not a conn.Conn): %s", psc)
	}
	return (*Conn)(psc), nil
}

// wrapConns returns a *Conn for all these ps.Conns
func wrapConns(conns1 []*ps.Conn) []*Conn {
	conns2 := make([]*Conn, len(conns1))
	for i, c1 := range conns1 {
		if c2, err := wrapConn(c1); err == nil {
			conns2[i] = c2
		}
	}
	return conns2
}

// newConnSetup does the swarm's "setup" for a connection. returns the underlying
// conn.Conn this method is used by both swarm.Dial and ps.Swarm connHandler
func (s *Swarm) newConnSetup(ctx context.Context, psConn *ps.Conn) (*Conn, error) {

	// wrap with a Conn
	sc, err := wrapConn(psConn)
	if err != nil {
		return nil, err
	}

	// removing this for now, as it has to change. we can put this in a different
	// sub-protocol anyway.
	// // run Handshake3
	// if err := runHandshake3(ctx, s, sc); err != nil {
	// 	return nil, err
	// }

	// ok great! we can use it. add it to our group.

	// set the RemotePeer as a group on the conn. this lets us group
	// connections in the StreamSwarm by peer, and get a streams from
	// any available connection in the group (better multiconn):
	//   swarm.StreamSwarm().NewStreamWithGroup(remotePeer)
	psConn.AddGroup(sc.RemotePeer())

	return sc, nil
}

// func runHandshake3(ctx context.Context, s *Swarm, c *Conn) error {
// 	log.Event(ctx, "newConnection", c.LocalPeer(), c.RemotePeer())

// 	stream, err := c.NewStream()
// 	if err != nil {
// 		return err
// 	}

// 	// handshake3 (this whole thing is ugly. maybe lets get rid of it...)
// 	h3result, err := conn.Handshake3(ctx, stream, c.RawConn())
// 	if err != nil {
// 		return fmt.Errorf("Handshake3 failed: %s", err)
// 	}

// 	// check for nats. you know, just in case.
// 	if h3result.LocalObservedAddress != nil {
// 		checkNATWarning(s, h3result.LocalObservedAddress, c.LocalMultiaddr())
// 	} else {
// 		log.Warningf("Received nil observed address from %s", c.RemotePeer())
// 	}

// 	stream.Close()
// 	log.Event(ctx, "handshake3Succeeded", c.LocalPeer(), c.RemotePeer())
// 	return nil
// }
