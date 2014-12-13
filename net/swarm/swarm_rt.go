package swarm

import (
	"sync"

	conn "github.com/jbenet/go-ipfs/net/conn"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

// swarmRoutingTable collects the peers
type swarmRoutingTable struct {
	local  peer.Peer
	client router.Node
	peers  map[u.Key]*swarmPeer
	sync.RWMutex
}

func newRoutingTable(local peer.Peer, client router.Node) *swarmRoutingTable {
	return &swarmRoutingTable{
		local:  local,
		client: client,
		peers:  map[u.Key]*swarmPeer{},
	}
}

func (rt *swarmRoutingTable) getOrAdd(s *Swarm, p peer.Peer) (*swarmPeer, error) {
	rt.Lock()
	defer rt.Unlock()

	sp, ok := rt.peers[p.Key()]
	if ok {
		return sp, nil
	}

	// newSwarmPeer is what kicks off the reader goroutines.
	sp, err := newSwarmPeer(s, p)
	if err != nil {
		return nil, err
	}
	rt.peers[p.Key()] = sp
	return sp, nil
}

func (rt *swarmRoutingTable) remove(p peer.Peer) *swarmPeer {
	rt.Lock()
	defer rt.Unlock()
	sp, ok := rt.peers[p.Key()]
	if ok {
		delete(rt.peers, p.Key())
	}
	return sp
}

func (rt *swarmRoutingTable) getByID(pid peer.ID) *swarmPeer {
	rt.RLock()
	defer rt.RUnlock()
	return rt.peers[u.Key(pid)]
}

func (rt *swarmRoutingTable) get(p peer.Peer) *swarmPeer {
	rt.RLock()
	defer rt.RUnlock()
	return rt.peers[p.Key()]
}

func (rt *swarmRoutingTable) connList() []conn.Conn {
	rt.RLock()
	defer rt.RUnlock()

	var out []conn.Conn
	for _, sp := range rt.peers {
		out = append(out, sp.conn)
	}
	return out
}

func (rt *swarmRoutingTable) peerList() []peer.Peer {
	rt.RLock()
	defer rt.RUnlock()

	var out []peer.Peer
	for _, sp := range rt.peers {
		out = append(out, sp.RemotePeer())
	}
	return out
}

// Route implements routing.Route
func (rt *swarmRoutingTable) Route(p router.Packet) router.Node {

	// no need to lock :)
	if p.Destination() == rt.client.Address() {
		// log.Debugf("%s swarmRoutingTable route %s to client %s ? ", p.Destination(), rt.client.Address(), p.Payload())
		return rt.client
	}

	rt.RLock()
	defer rt.RUnlock()

	for _, sp := range rt.peers {
		if sp.RemotePeer() == p.Destination() {
			// log.Debugf("%s swarmRoutingTable route %s to peer %s ? ", p.Destination(), sp.RemotePeer(), p.Payload())
			return sp
		}
	}

	return nil // no route
}

func (s *Swarm) client() router.Node {
	return s.rt.client
}

func (s *Swarm) addConn(c conn.Conn) error {
	sp, err := s.rt.getOrAdd(s, c.RemotePeer())
	if err != nil {
		return err
	}

	sp.conn.Add(c)
	return nil
}

func (s *Swarm) closeConn(p peer.Peer) error {
	sp := s.rt.remove(p)
	if sp == nil {
		return nil
	}

	return sp.Close()
}

// HandlePacket routes messages out through connections, or to the client
func (s *Swarm) HandlePacket(p router.Packet, from router.Node) error {
	msg, ok := p.Payload().([]byte)
	if !ok {
		return netmsg.ErrInvalidPayload
	}

	if len(msg) >= conn.MaxMessageSize {
		log.Criticalf("Attempted to send message bigger than max size. (%d)", len(msg))
	}

	next := s.rt.Route(p)
	if next == nil {
		// log.Debugf("%s swarm HandlePacket %s -> %s -> %s -> %s: %s",
		// 	s.local, from.Address(), s.Address(), "????", p.Destination(), p.Payload())
		return router.ErrNoRoute
	}

	// log.Debugf("%s swarm HandlePacket %s -> %s -> %s -> %s: %s",
	// 	s.local, from.Address(), s.Address(), next.Address(), p.Destination(), p.Payload())

	return next.HandlePacket(p, s)
}
