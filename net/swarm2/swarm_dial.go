package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	peer "github.com/jbenet/go-ipfs/peer"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
func (s *Swarm) Dial(ctx context.Context, p peer.Peer) (*Conn, error) {

	if p.ID().Equal(s.local.ID()) {
		return nil, errors.New("Attempted connection to self!")
	}

	// check if we already have an open connection first
	cs := s.ConnectionsToPeer(p)
	for _, c := range cs {
		if c != nil { // dump out the first one we find
			return c, nil
		}
	}

	// check if we don't have the peer in Peerstore
	p, err := s.peers.Add(p)
	if err != nil {
		return nil, err
	}

	// open connection to peer
	d := &conn.Dialer{
		LocalPeer: s.local,
		Peerstore: s.peers,
	}

	if len(p.Addresses()) == 0 {
		return nil, errors.New("peer has no addresses")
	}

	// try to connect to one of the peer's known addresses.
	// for simplicity, we do this sequentially.
	// A future commit will do this asynchronously.
	var connC conn.Conn
	for _, addr := range p.Addresses() {
		connC, err = d.DialAddr(ctx, addr, p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	// ok try to setup the new connection.
	swarmC, err := dialConnSetup(ctx, s, connC)
	if err != nil {
		log.Error("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		swarmC.Close() // close the connection. didn't work out :(
		return nil, err
	}

	log.Event(ctx, "dial", p)
	return swarmC, nil
}

// dialConnSetup is the setup logic for a connection from the dial side. it
// needs to add the Conn to the StreamSwarm, then run newConnSetup
func dialConnSetup(ctx context.Context, s *Swarm, connC conn.Conn) (*Conn, error) {

	psC, err := s.swarm.AddConn(connC)
	if err != nil {
		// connC is closed by caller if we fail.
		return nil, fmt.Errorf("failed to add conn to ps.Swarm: %s", err)
	}

	// ok try to setup the new connection. (newConnSetup will add to group)
	swarmC, err := s.newConnSetup(ctx, psC)
	if err != nil {
		log.Error("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		swarmC.Close() // we need to call this to make sure psC is Closed.
		return nil, err
	}

	return swarmC, err
}
