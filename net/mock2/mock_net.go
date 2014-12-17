package mocknet

import (
	"fmt"
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
)

type peerID string

// mocknet implements mocknet.Mocknet
type mocknet struct {
	// must map on peer.ID (instead of peer.Peer) because
	// each inet.Network has different peerstore
	nets map[peerID]*peernet

	// links make it possible to connect two peers.
	// think of links as the physical medium.
	// usually only one, but there could be multiple
	// **links are shared between peers**
	links map[peerID]map[peerID]map[*link]struct{}

	linkDefaults LinkOptions

	cg ctxgroup.ContextGroup // for Context closing
	sync.RWMutex
}

func New(ctx context.Context) Mocknet {
	return &mocknet{
		nets:  map[peerID]*peernet{},
		links: map[peerID]map[peerID]map[*link]struct{}{},
		cg:    ctxgroup.WithContext(ctx),
	}
}

func (mn *mocknet) GenPeer() (inet.Network, error) {
	p, err := testutil.PeerWithNewKeys()
	if err != nil {
		return nil, err
	}

	n, err := mn.AddPeer(p.ID())
	if err != nil {
		return nil, err
	}

	// copy over keys
	if err := n.LocalPeer().Update(p); err != nil {
		return nil, err
	}

	return n, nil
}

func (mn *mocknet) AddPeer(p peer.ID) (inet.Network, error) {
	n, err := newPeernet(mn.cg.Context(), mn, p)
	if err != nil {
		return nil, err
	}

	mn.cg.AddChildGroup(n.cg)

	mn.Lock()
	mn.nets[pid(n.peer)] = n
	mn.Unlock()
	return n, nil
}

func (mn *mocknet) Peer(pid peer.ID) peer.Peer {
	mn.RLock()
	defer mn.RUnlock()

	for _, n := range mn.nets {
		if n.peer.ID().Equal(pid) {
			return n.peer
		}
	}
	return nil
}

func (mn *mocknet) Peers() []peer.Peer {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]peer.Peer, 0, len(mn.nets))
	for _, n := range mn.nets {
		cp = append(cp, n.peer)
	}
	return cp
}

func (mn *mocknet) Net(pid peer.ID) inet.Network {
	mn.RLock()
	defer mn.RUnlock()

	for _, n := range mn.nets {
		if n.peer.ID().Equal(pid) {
			return n
		}
	}
	return nil
}

func (mn *mocknet) Nets() []inet.Network {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]inet.Network, 0, len(mn.nets))
	for _, n := range mn.nets {
		cp = append(cp, n)
	}
	return cp
}

func (mn *mocknet) LinkAll() error {
	nets := mn.Nets()
	for _, n1 := range nets {
		for _, n2 := range nets {
			if _, err := mn.LinkNets(n1, n2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mn *mocknet) LinkPeers(p1, p2 peer.Peer) (Link, error) {
	mn.RLock()
	n1 := mn.nets[pid(p1)]
	n2 := mn.nets[pid(p2)]
	mn.RUnlock()

	if n1 == nil {
		return nil, fmt.Errorf("network for p1 not in mocknet")
	}

	if n2 == nil {
		return nil, fmt.Errorf("network for p2 not in mocknet")
	}

	return mn.LinkNets(n1, n2)
}

func (mn *mocknet) validate(n inet.Network) (*peernet, error) {
	// WARNING: assumes locks acquired

	nr, ok := n.(*peernet)
	if !ok {
		return nil, fmt.Errorf("Network not supported (use mock package nets only)")
	}

	if _, found := mn.nets[pid(nr.peer)]; !found {
		return nil, fmt.Errorf("Network not on mocknet. is it from another mocknet?")
	}

	return nr, nil
}

func (mn *mocknet) LinkNets(n1, n2 inet.Network) (Link, error) {
	mn.Lock()
	defer mn.Unlock()

	if _, err := mn.validate(n1); err != nil {
		return nil, err
	}

	if _, err := mn.validate(n2); err != nil {
		return nil, err
	}

	l := newLink(mn)
	mn.addLink(l)
	return l, nil
}

func (mn *mocknet) Unlink(l2 Link) error {

	l, ok := l2.(*link)
	if !ok {
		return fmt.Errorf("only links from mocknet are supported")
	}

	mn.removeLink(l)
	return nil
}

func (mn *mocknet) UnlinkPeers(p1, p2 peer.Peer) error {
	ls := mn.LinksBetweenPeers(p1, p2)
	if ls == nil {
		return fmt.Errorf("no link between p1 and p2")
	}

	for _, l := range ls {
		if err := mn.Unlink(l); err != nil {
			return err
		}
	}
	return nil
}

func (mn *mocknet) UnlinkNets(n1, n2 inet.Network) error {
	return mn.UnlinkPeers(n1.LocalPeer(), n2.LocalPeer())
}

func (mn *mocknet) addLink(l *link) {
	mn.Lock()
	defer mn.Unlock()

	n1, n2 := l.nets[0], l.nets[1]
	mn.links[pid(n1.peer)][pid(n2.peer)][l] = struct{}{}
	mn.links[pid(n2.peer)][pid(n1.peer)][l] = struct{}{}
}

func (mn *mocknet) removeLink(l *link) {
	mn.Lock()
	defer mn.Unlock()

	n1, n2 := l.nets[0], l.nets[1]
	delete(mn.links[pid(n1.peer)][pid(n2.peer)], l)
	delete(mn.links[pid(n2.peer)][pid(n1.peer)], l)
}

func (mn *mocknet) ConnectAll() error {
	nets := mn.Nets()
	for _, n1 := range nets {
		for _, n2 := range nets {
			if n1 == n2 {
				continue
			}

			if err := mn.ConnectNets(n1, n2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mn *mocknet) ConnectPeers(a, b peer.Peer) error {
	return mn.Net(a.ID()).DialPeer(mn.cg.Context(), b)
}

func (mn *mocknet) ConnectNets(a, b inet.Network) error {
	return a.DialPeer(mn.cg.Context(), b.LocalPeer())
}

func (mn *mocknet) DisconnectPeers(p1, p2 peer.Peer) error {
	return mn.Net(p1.ID()).ClosePeer(p2)
}

func (mn *mocknet) DisconnectNets(n1, n2 inet.Network) error {
	return n1.ClosePeer(n2.LocalPeer())
}

func (mn *mocknet) LinksBetweenPeers(p1, p2 peer.Peer) []Link {
	mn.RLock()
	defer mn.RUnlock()

	ls1, found := mn.links[pid(p1)]
	if !found {
		return nil
	}

	ls2, found := ls1[pid(p2)]
	if !found {
		return nil
	}

	cp := make([]Link, 0, len(ls2))
	for l := range ls2 {
		cp = append(cp, l)
	}
	return cp
}

func (mn *mocknet) LinksBetweenNets(n1, n2 inet.Network) []Link {
	return mn.LinksBetweenPeers(n1.LocalPeer(), n2.LocalPeer())
}

func (mn *mocknet) SetLinkDefaults(o LinkOptions) {
	mn.Lock()
	mn.linkDefaults = o
	mn.Unlock()
}

func (mn *mocknet) LinkDefaults() LinkOptions {
	mn.RLock()
	defer mn.RUnlock()
	return mn.linkDefaults
}
