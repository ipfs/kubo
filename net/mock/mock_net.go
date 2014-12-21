package mocknet

import (
	"fmt"
	"sync"

	ic "github.com/jbenet/go-ipfs/crypto"
	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// mocknet implements mocknet.Mocknet
type mocknet struct {
	// must map on peer.ID (instead of peer.ID) because
	// each inet.Network has different peerstore
	nets map[peer.ID]*peernet

	// links make it possible to connect two peers.
	// think of links as the physical medium.
	// usually only one, but there could be multiple
	// **links are shared between peers**
	links map[peer.ID]map[peer.ID]map[*link]struct{}

	linkDefaults LinkOptions

	cg ctxgroup.ContextGroup // for Context closing
	sync.RWMutex
}

func New(ctx context.Context) Mocknet {
	return &mocknet{
		nets:  map[peer.ID]*peernet{},
		links: map[peer.ID]map[peer.ID]map[*link]struct{}{},
		cg:    ctxgroup.WithContext(ctx),
	}
}

func (mn *mocknet) GenPeer() (inet.Network, error) {
	sk, _, err := testutil.RandKeyPair(512)
	if err != nil {
		return nil, err
	}

	a := testutil.RandLocalTCPAddress()

	n, err := mn.AddPeer(sk, a)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (mn *mocknet) AddPeer(k ic.PrivKey, a ma.Multiaddr) (inet.Network, error) {
	n, err := newPeernet(mn.cg.Context(), mn, k, a)
	if err != nil {
		return nil, err
	}

	mn.cg.AddChildGroup(n.cg)

	mn.Lock()
	mn.nets[n.peer] = n
	mn.Unlock()
	return n, nil
}

func (mn *mocknet) Peers() []peer.ID {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]peer.ID, 0, len(mn.nets))
	for _, n := range mn.nets {
		cp = append(cp, n.peer)
	}
	return cp
}

func (mn *mocknet) Net(pid peer.ID) inet.Network {
	mn.RLock()
	defer mn.RUnlock()

	for _, n := range mn.nets {
		if n.peer == pid {
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

// Links returns a copy of the internal link state map.
// (wow, much map. so data structure. how compose. ahhh pointer)
func (mn *mocknet) Links() LinkMap {
	mn.RLock()
	defer mn.RUnlock()

	links := map[string]map[string]map[Link]struct{}{}
	for p1, lm := range mn.links {
		sp1 := string(p1)
		links[sp1] = map[string]map[Link]struct{}{}
		for p2, ls := range lm {
			sp2 := string(p2)
			links[sp1][sp2] = map[Link]struct{}{}
			for l := range ls {
				links[sp1][sp2][l] = struct{}{}
			}
		}
	}
	return links
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

func (mn *mocknet) LinkPeers(p1, p2 peer.ID) (Link, error) {
	mn.RLock()
	n1 := mn.nets[p1]
	n2 := mn.nets[p2]
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

	if _, found := mn.nets[nr.peer]; !found {
		return nil, fmt.Errorf("Network not on mocknet. is it from another mocknet?")
	}

	return nr, nil
}

func (mn *mocknet) LinkNets(n1, n2 inet.Network) (Link, error) {
	mn.RLock()
	n1r, err1 := mn.validate(n1)
	n2r, err2 := mn.validate(n2)
	ld := mn.linkDefaults
	mn.RUnlock()

	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	l := newLink(mn, ld)
	l.nets = append(l.nets, n1r, n2r)
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

func (mn *mocknet) UnlinkPeers(p1, p2 peer.ID) error {
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

// get from the links map. and lazily contruct.
func (mn *mocknet) linksMapGet(p1, p2 peer.ID) *map[*link]struct{} {

	l1, found := mn.links[p1]
	if !found {
		mn.links[p1] = map[peer.ID]map[*link]struct{}{}
		l1 = mn.links[p1] // so we make sure it's there.
	}

	l2, found := l1[p2]
	if !found {
		m := map[*link]struct{}{}
		l1[p2] = m
		l2 = l1[p2]
	}

	return &l2
}

func (mn *mocknet) addLink(l *link) {
	mn.Lock()
	defer mn.Unlock()

	n1, n2 := l.nets[0], l.nets[1]
	(*mn.linksMapGet(n1.peer, n2.peer))[l] = struct{}{}
	(*mn.linksMapGet(n2.peer, n1.peer))[l] = struct{}{}
}

func (mn *mocknet) removeLink(l *link) {
	mn.Lock()
	defer mn.Unlock()

	n1, n2 := l.nets[0], l.nets[1]
	delete(*mn.linksMapGet(n1.peer, n2.peer), l)
	delete(*mn.linksMapGet(n2.peer, n1.peer), l)
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

func (mn *mocknet) ConnectPeers(a, b peer.ID) error {
	return mn.Net(a).DialPeer(mn.cg.Context(), b)
}

func (mn *mocknet) ConnectNets(a, b inet.Network) error {
	return a.DialPeer(mn.cg.Context(), b.LocalPeer())
}

func (mn *mocknet) DisconnectPeers(p1, p2 peer.ID) error {
	return mn.Net(p1).ClosePeer(p2)
}

func (mn *mocknet) DisconnectNets(n1, n2 inet.Network) error {
	return n1.ClosePeer(n2.LocalPeer())
}

func (mn *mocknet) LinksBetweenPeers(p1, p2 peer.ID) []Link {
	mn.RLock()
	defer mn.RUnlock()

	ls2 := *mn.linksMapGet(p1, p2)
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
