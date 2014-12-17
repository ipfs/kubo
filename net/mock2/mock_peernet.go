package mocknet

import (
	"container/list"
	"fmt"
	"math/rand"
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// peernet implements inet.Network
type peernet struct {
	mocknet *mocknet // parent

	peer peer.Peer
	ps   peer.Peerstore

	// conns are actual live connections between peers.
	// many conns could run over each link.
	// **conns are NOT shared between peers**
	connsByPeer map[peerID]list.List
	connsByLink map[*link]list.List

	// needed to implement inet.Network
	mux inet.Mux

	cg ctxgroup.ContextGroup
	sync.RWMutex
}

// newPeernet constructs a new peernet
func newPeernet(ctx context.Context, m *mocknet, id peer.ID) (*peernet, error) {

	// create our own entirely, so that peers dont get shuffled across
	// network divides. dont share peers.
	ps := peer.NewPeerstore()
	p, err := ps.FindOrCreate(id)
	if err != nil {
		return nil, err
	}

	n := &peernet{
		mocknet: m,
		peer:    p,
		ps:      ps,
		mux:     inet.Mux{Handlers: inet.StreamHandlerMap{}},
		cg:      ctxgroup.WithContext(ctx),

		connsByPeer: map[peerID]list.List{},
		connsByLink: map[*link]list.List{},
	}

	n.cg.SetTeardown(n.teardown)
	return n, nil
}

func (pn *peernet) teardown() error {

	// close the connections
	for _, c := range pn.allConns() {
		c.Close()
	}
	return nil
}

// allConns returns all the connections between this peer and others
func (pn *peernet) allConns() []*conn {
	pn.RLock()
	var cs []*conn
	for _, csl := range pn.connsByPeer {
		for e := csl.Front(); e != nil; e = e.Next() {
			c := e.Value.(*conn)
			cs = append(cs, c)
		}
	}
	pn.RUnlock()
	return cs
}

// Close calls the ContextCloser func
func (pn *peernet) Close() error {
	return pn.cg.Close()
}

func (pn *peernet) String() string {
	return fmt.Sprintf("<mock.peernet %s - %d conns>", pn.peer, len(pn.allConns()))
}

// handleNewStream is an internal function to trigger the muxer handler
func (pn *peernet) handleNewStream(s inet.Stream) {
	go pn.mux.Handle(s)
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (pn *peernet) DialPeer(ctx context.Context, p peer.Peer) error {
	return pn.connect(p)
}

func (pn *peernet) connect(p peer.Peer) error {
	// cannot trust the peer we get. typical for tests to give us
	// a peer from some other peerstore...
	p, err := pn.ps.Add(p)
	if err != nil {
		return err
	}

	// first, check if we already have live connections
	pn.RLock()
	cs, found := pn.connsByPeer[pid(p)]
	ncs := cs.Len()
	pn.RUnlock()
	if found && ncs > 0 {
		return nil
	}

	// ok, must create a new connection. we need a link
	links := pn.mocknet.LinksBetweenPeers(pn.peer, p)
	if len(links) < 1 {
		return fmt.Errorf("cannot connect to peer %s", p)
	}

	// if many links found, how do we select? for now, randomly...
	// this would be an interesting place to test logic that can measure
	// links (network interfaces) and select properly
	l := links[rand.Intn(len(links))]

	// create a new connection with link
	pn.openConn(p, l.(*link))
	return nil
}

func (pn *peernet) openConn(r peer.Peer, l *link) *conn {
	lc, rc := l.newConnPair()
	pn.addConn(lc)
	rc.net.remoteOpenedConn(rc)
	return lc
}

func (pn *peernet) remoteOpenedConn(c *conn) {
	pn.addConn(c)
}

// addConn constructs and adds a connection
// to given remote peer over given link
func (pn *peernet) addConn(c *conn) {
	pn.Lock()
	cs, found := pn.connsByPeer[pid(c.RemotePeer())]
	if !found {
		cs = list.List{}
		pn.connsByPeer[pid(c.RemotePeer())] = cs
	}
	cs.PushBack(c)

	cs, found = pn.connsByLink[c.link]
	if !found {
		cs = list.List{}
		pn.connsByLink[c.link] = cs
	}
	cs.PushBack(c)
	pn.Unlock()
}

// removeConn removes a given conn
func (pn *peernet) removeConn(c *conn) {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByLink[c.link]
	if !found {
		panic("attempting to remove a conn that doesnt exist")
	}

	for e := cs.Front(); e != nil; e = e.Next() {
		if c == e.Value {
			cs.Remove(e)
			break
		}
	}

	cs, found = pn.connsByPeer[pid(c.remote)]
	if !found {
		panic("attempting to remove a conn that doesnt exist")
	}

	for e := cs.Front(); e != nil; e = e.Next() {
		if c == e.Value {
			cs.Remove(e)
			break
		}
	}
}

// CtxGroup returns the network's ContextGroup
func (pn *peernet) CtxGroup() ctxgroup.ContextGroup {
	return pn.cg
}

// LocalPeer the network's LocalPeer
func (pn *peernet) LocalPeer() peer.Peer {
	return pn.peer
}

// Peers returns the connected peers
func (pn *peernet) Peers() []peer.Peer {
	pn.RLock()
	defer pn.RUnlock()

	peers := make([]peer.Peer, 0, len(pn.connsByPeer))
	for _, cs := range pn.connsByPeer {
		if cs.Len() == 0 {
			panic("found empty connection list. not removed properly...")
		}

		c := cs.Front().Value.(*conn)
		peers = append(peers, c.remote)
	}
	return peers
}

// Conns returns all the connections of this peer
func (pn *peernet) Conns() []inet.Conn {
	pn.RLock()
	defer pn.RUnlock()

	out := make([]inet.Conn, 0, len(pn.connsByPeer))
	for _, cs := range pn.connsByPeer {
		for e := cs.Front(); e != nil; e = e.Next() {
			c := e.Value.(*conn)
			out = append(out, c)
		}
	}
	return out
}

func (pn *peernet) ConnsToPeer(p peer.Peer) []inet.Conn {
	pn.RLock()
	defer pn.RUnlock()

	cs, found := pn.connsByPeer[pid(p)]
	if !found {
		return nil
	}
	if cs.Len() == 0 {
		panic("found empty connection list. not removed properly...")
	}

	var cs2 []inet.Conn
	for e := cs.Front(); e != nil; e = e.Next() {
		c := e.Value.(*conn)
		cs2 = append(cs2, c)
	}
	return cs2
}

// ClosePeer connections to peer
func (pn *peernet) ClosePeer(p peer.Peer) error {
	pn.RLock()
	cs, found := pn.connsByPeer[pid(p)]
	pn.RUnlock()
	if !found {
		return nil
	}

	for e := cs.Front(); e != nil; e = e.Next() {
		c := e.Value.(*conn)
		pn.closeConn(c)
	}
	return nil
}

func (pn *peernet) closeConn(c *conn) {
	pn.Lock()
	defer pn.Unlock()

	// remove it from connsByPeer
	cs, found := pn.connsByPeer[pid(c.remote)]
	if !found {
		panic("attempted to close connection that doesnt exist! (peer)")
	}

	for e := cs.Front(); e != nil; e = e.Next() {
		if c == e.Value.(*conn) {
			cs.Remove(e)
		}
	}
	if cs.Len() == 0 {
		delete(pn.connsByPeer, pid(c.remote))
	}

	// remove it from connsByLink
	cs, found = pn.connsByLink[c.link]
	if !found {
		panic("attempted to close connection that doesnt exist! (link)")
	}
	for e := cs.Front(); e != nil; e = e.Next() {
		if c == e.Value.(*conn) {
			cs.Remove(e)
		}
	}
	if cs.Len() == 0 {
		delete(pn.connsByLink, c.link)
	}
}

// BandwidthTotals returns the total amount of bandwidth transferred
func (pn *peernet) BandwidthTotals() (in uint64, out uint64) {
	// need to implement this. probably best to do it in swarm this time.
	// need a "metrics" object
	return 0, 0
}

// ListenAddresses returns a list of addresses at which this network listens.
func (pn *peernet) ListenAddresses() []ma.Multiaddr {
	return []ma.Multiaddr{}
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (pn *peernet) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return []ma.Multiaddr{}, nil
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connecter || NotConnected. Expand into more later.
func (pn *peernet) Connectedness(p peer.Peer) inet.Connectedness {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByPeer[pid(p)]
	if found {
		if cs.Len() == 0 {
			panic("found empty connection list. not removed properly...")
		}

		return inet.Connected
	}
	return inet.NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (pn *peernet) NewStream(pr inet.ProtocolID, p peer.Peer) (inet.Stream, error) {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByPeer[pid(p)]
	if !found {
		return nil, fmt.Errorf("no connection to peer")
	}

	// if many conns are found, how do we select? for now, randomly...
	// this would be an interesting place to test logic that can measure
	// links (network interfaces) and select properly
	c := randomListElem(&cs).Value.(*conn)

	return c.NewStreamWithProtocol(pr, p)
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (pn *peernet) SetHandler(p inet.ProtocolID, h inet.StreamHandler) {
	pn.mux.SetHandler(p, h)
}

func pid(p peer.Peer) peerID {
	return peerID(p.ID())
}

func randomListElem(l *list.List) *list.Element {
	n := rand.Intn(l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		if n == 0 {
			return e
		}
		n--
	}

	panic("unreachable")
}
