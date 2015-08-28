package mocknet

import (
	"fmt"
	"sort"
	"sync"

	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	host "github.com/ipfs/go-ipfs/p2p/host"
	bhost "github.com/ipfs/go-ipfs/p2p/host/basic"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	p2putil "github.com/ipfs/go-ipfs/p2p/test/util"
	testutil "github.com/ipfs/go-ipfs/util/testutil"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	goprocessctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

// mocknet implements mocknet.Mocknet
type mocknet struct {
	nets  map[peer.ID]*peernet
	hosts map[peer.ID]*bhost.BasicHost

	// links make it possible to connect two peers.
	// think of links as the physical medium.
	// usually only one, but there could be multiple
	// **links are shared between peers**
	links map[peer.ID]map[peer.ID]map[*link]struct{}

	linkDefaults LinkOptions

	proc goprocess.Process // for Context closing
	ctx  context.Context
	sync.RWMutex
}

func New(ctx context.Context) Mocknet {
	return &mocknet{
		nets:  map[peer.ID]*peernet{},
		hosts: map[peer.ID]*bhost.BasicHost{},
		links: map[peer.ID]map[peer.ID]map[*link]struct{}{},
		proc:  goprocessctx.WithContext(ctx),
		ctx:   ctx,
	}
}

func (mn *mocknet) GenPeer() (host.Host, error) {
	sk, err := p2putil.RandTestBogusPrivateKey()
	if err != nil {
		return nil, err
	}

	a := testutil.RandLocalTCPAddress()

	h, err := mn.AddPeer(sk, a)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (mn *mocknet) AddPeer(k ic.PrivKey, a ma.Multiaddr) (host.Host, error) {
	p, err := peer.IDFromPublicKey(k.GetPublic())
	if err != nil {
		return nil, err
	}

	ps := peer.NewPeerstore()
	ps.AddAddr(p, a, peer.PermanentAddrTTL)
	ps.AddPrivKey(p, k)
	ps.AddPubKey(p, k.GetPublic())

	return mn.AddPeerWithPeerstore(p, ps)
}

func (mn *mocknet) AddPeerWithPeerstore(p peer.ID, ps peer.Peerstore) (host.Host, error) {
	n, err := newPeernet(mn.ctx, mn, p, ps)
	if err != nil {
		return nil, err
	}

	h := bhost.New(n)

	mn.proc.AddChild(n.proc)

	mn.Lock()
	mn.nets[n.peer] = n
	mn.hosts[n.peer] = h
	mn.Unlock()
	return h, nil
}

func (mn *mocknet) Peers() []peer.ID {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]peer.ID, 0, len(mn.nets))
	for _, n := range mn.nets {
		cp = append(cp, n.peer)
	}
	sort.Sort(peer.IDSlice(cp))
	return cp
}

func (mn *mocknet) Host(pid peer.ID) host.Host {
	mn.RLock()
	host := mn.hosts[pid]
	mn.RUnlock()
	return host
}

func (mn *mocknet) Net(pid peer.ID) inet.Network {
	mn.RLock()
	n := mn.nets[pid]
	mn.RUnlock()
	return n
}

func (mn *mocknet) Hosts() []host.Host {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]host.Host, 0, len(mn.hosts))
	for _, h := range mn.hosts {
		cp = append(cp, h)
	}

	sort.Sort(hostSlice(cp))
	return cp
}

func (mn *mocknet) Nets() []inet.Network {
	mn.RLock()
	defer mn.RUnlock()

	cp := make([]inet.Network, 0, len(mn.nets))
	for _, n := range mn.nets {
		cp = append(cp, n)
	}
	sort.Sort(netSlice(cp))
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

func (mn *mocknet) ConnectAllButSelf() error {
	nets := mn.Nets()
	for _, n1 := range nets {
		for _, n2 := range nets {
			if n1 == n2 {
				continue
			}

			if _, err := mn.ConnectNets(n1, n2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mn *mocknet) ConnectPeers(a, b peer.ID) (inet.Conn, error) {
	return mn.Net(a).DialPeer(mn.ctx, b)
}

func (mn *mocknet) ConnectNets(a, b inet.Network) (inet.Conn, error) {
	return a.DialPeer(mn.ctx, b.LocalPeer())
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

// netSlice for sorting by peer
type netSlice []inet.Network

func (es netSlice) Len() int           { return len(es) }
func (es netSlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es netSlice) Less(i, j int) bool { return string(es[i].LocalPeer()) < string(es[j].LocalPeer()) }

// hostSlice for sorting by peer
type hostSlice []host.Host

func (es hostSlice) Len() int           { return len(es) }
func (es hostSlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es hostSlice) Less(i, j int) bool { return string(es[i].ID()) < string(es[j].ID()) }
