package mocknet

import (
	"fmt"
	"math/rand"
	"sync"

	ic "github.com/jbenet/go-ipfs/crypto"
	inet "github.com/jbenet/go-ipfs/net"
	ids "github.com/jbenet/go-ipfs/net/services/identify"
	mux "github.com/jbenet/go-ipfs/net/services/mux"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// peernet implements inet.Network
type peernet struct {
	mocknet *mocknet // parent

	peer peer.ID
	ps   peer.Peerstore

	// conns are actual live connections between peers.
	// many conns could run over each link.
	// **conns are NOT shared between peers**
	connsByPeer map[peer.ID]map[*conn]struct{}
	connsByLink map[*link]map[*conn]struct{}

	// needed to implement inet.Network
	mux mux.Mux
	ids *ids.IDService

	cg ctxgroup.ContextGroup
	sync.RWMutex
}

// newPeernet constructs a new peernet
func newPeernet(ctx context.Context, m *mocknet, k ic.PrivKey,
	a ma.Multiaddr) (*peernet, error) {

	p, err := peer.IDFromPublicKey(k.GetPublic())
	if err != nil {
		return nil, err
	}

	// create our own entirely, so that peers knowledge doesn't get shared
	ps := peer.NewPeerstore()
	ps.AddAddress(p, a)
	ps.AddPrivKey(p, k)
	ps.AddPubKey(p, k.GetPublic())

	n := &peernet{
		mocknet: m,
		peer:    p,
		ps:      ps,
		mux:     mux.Mux{Handlers: inet.StreamHandlerMap{}},
		cg:      ctxgroup.WithContext(ctx),

		connsByPeer: map[peer.ID]map[*conn]struct{}{},
		connsByLink: map[*link]map[*conn]struct{}{},
	}

	n.cg.SetTeardown(n.teardown)

	// setup a conn handler that immediately "asks the other side about them"
	// this is ProtocolIdentify.
	n.ids = ids.NewIDService(n)

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
		for c := range csl {
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

func (pn *peernet) Protocols() []inet.ProtocolID {
	return pn.mux.Protocols()
}

func (pn *peernet) Peerstore() peer.Peerstore {
	return pn.ps
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
func (pn *peernet) DialPeer(ctx context.Context, p peer.ID) error {
	return pn.connect(p)
}

func (pn *peernet) connect(p peer.ID) error {
	// first, check if we already have live connections
	pn.RLock()
	cs, found := pn.connsByPeer[p]
	pn.RUnlock()
	if found && len(cs) > 0 {
		return nil
	}

	log.Debugf("%s (newly) dialing %s", pn.peer, p)

	// ok, must create a new connection. we need a link
	links := pn.mocknet.LinksBetweenPeers(pn.peer, p)
	if len(links) < 1 {
		return fmt.Errorf("%s cannot connect to %s", pn.peer, p)
	}

	// if many links found, how do we select? for now, randomly...
	// this would be an interesting place to test logic that can measure
	// links (network interfaces) and select properly
	l := links[rand.Intn(len(links))]

	log.Debugf("%s dialing %s openingConn", pn.peer, p)
	// create a new connection with link
	pn.openConn(p, l.(*link))
	return nil
}

func (pn *peernet) openConn(r peer.ID, l *link) *conn {
	lc, rc := l.newConnPair(pn)
	log.Debugf("%s opening connection to %s", pn.LocalPeer(), lc.RemotePeer())
	pn.addConn(lc)
	rc.net.remoteOpenedConn(rc)
	return lc
}

func (pn *peernet) remoteOpenedConn(c *conn) {
	log.Debugf("%s accepting connection from %s", pn.LocalPeer(), c.RemotePeer())
	pn.addConn(c)
}

// addConn constructs and adds a connection
// to given remote peer over given link
func (pn *peernet) addConn(c *conn) {

	// run the Identify protocol/handshake.
	pn.ids.IdentifyConn(c)

	pn.Lock()
	cs, found := pn.connsByPeer[c.RemotePeer()]
	if !found {
		cs = map[*conn]struct{}{}
		pn.connsByPeer[c.RemotePeer()] = cs
	}
	pn.connsByPeer[c.RemotePeer()][c] = struct{}{}

	cs, found = pn.connsByLink[c.link]
	if !found {
		cs = map[*conn]struct{}{}
		pn.connsByLink[c.link] = cs
	}
	pn.connsByLink[c.link][c] = struct{}{}
	pn.Unlock()
}

// removeConn removes a given conn
func (pn *peernet) removeConn(c *conn) {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByLink[c.link]
	if !found || len(cs) < 1 {
		panic("attempting to remove a conn that doesnt exist")
	}
	delete(cs, c)

	cs, found = pn.connsByPeer[c.remote]
	if !found {
		panic("attempting to remove a conn that doesnt exist")
	}
	delete(cs, c)
}

// CtxGroup returns the network's ContextGroup
func (pn *peernet) CtxGroup() ctxgroup.ContextGroup {
	return pn.cg
}

// LocalPeer the network's LocalPeer
func (pn *peernet) LocalPeer() peer.ID {
	return pn.peer
}

// Peers returns the connected peers
func (pn *peernet) Peers() []peer.ID {
	pn.RLock()
	defer pn.RUnlock()

	peers := make([]peer.ID, 0, len(pn.connsByPeer))
	for _, cs := range pn.connsByPeer {
		for c := range cs {
			peers = append(peers, c.remote)
			break
		}
	}
	return peers
}

// Conns returns all the connections of this peer
func (pn *peernet) Conns() []inet.Conn {
	pn.RLock()
	defer pn.RUnlock()

	out := make([]inet.Conn, 0, len(pn.connsByPeer))
	for _, cs := range pn.connsByPeer {
		for c := range cs {
			out = append(out, c)
		}
	}
	return out
}

func (pn *peernet) ConnsToPeer(p peer.ID) []inet.Conn {
	pn.RLock()
	defer pn.RUnlock()

	cs, found := pn.connsByPeer[p]
	if !found || len(cs) == 0 {
		return nil
	}

	var cs2 []inet.Conn
	for c := range cs {
		cs2 = append(cs2, c)
	}
	return cs2
}

// ClosePeer connections to peer
func (pn *peernet) ClosePeer(p peer.ID) error {
	pn.RLock()
	cs, found := pn.connsByPeer[p]
	pn.RUnlock()
	if !found {
		return nil
	}

	for c := range cs {
		c.Close()
	}
	return nil
}

// BandwidthTotals returns the total amount of bandwidth transferred
func (pn *peernet) BandwidthTotals() (in uint64, out uint64) {
	// need to implement this. probably best to do it in swarm this time.
	// need a "metrics" object
	return 0, 0
}

// ListenAddresses returns a list of addresses at which this network listens.
func (pn *peernet) ListenAddresses() []ma.Multiaddr {
	return pn.Peerstore().Addresses(pn.LocalPeer())
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (pn *peernet) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return pn.ListenAddresses(), nil
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connecter || NotConnected. Expand into more later.
func (pn *peernet) Connectedness(p peer.ID) inet.Connectedness {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByPeer[p]
	if found && len(cs) > 0 {
		return inet.Connected
	}
	return inet.NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (pn *peernet) NewStream(pr inet.ProtocolID, p peer.ID) (inet.Stream, error) {
	pn.Lock()
	defer pn.Unlock()

	cs, found := pn.connsByPeer[p]
	if !found || len(cs) < 1 {
		return nil, fmt.Errorf("no connection to peer")
	}

	// if many conns are found, how do we select? for now, randomly...
	// this would be an interesting place to test logic that can measure
	// links (network interfaces) and select properly
	n := rand.Intn(len(cs))
	var c *conn
	for c = range cs {
		if n == 0 {
			break
		}
		n--
	}

	return c.NewStreamWithProtocol(pr)
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (pn *peernet) SetHandler(p inet.ProtocolID, h inet.StreamHandler) {
	pn.mux.SetHandler(p, h)
}

func (pn *peernet) IdentifyProtocol() *ids.IDService {
	return pn.ids
}
