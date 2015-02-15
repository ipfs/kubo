package routedhost

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	routing "github.com/jbenet/go-ipfs/routing"
)

var log = eventlog.Logger("p2p/host/routed")

// AddressTTL is the expiry time for our addresses.
// We expire them quickly.
const AddressTTL = time.Second * 10

// RoutedHost is a p2p Host that includes a routing system.
// This allows the Host to find the addresses for peers when
// it does not have them.
type RoutedHost struct {
	host  host.Host // embedded other host.
	route routing.IpfsRouting
}

func Wrap(h host.Host, r routing.IpfsRouting) *RoutedHost {
	return &RoutedHost{h, r}
}

// Connect ensures there is a connection between this host and the peer with
// given peer.ID. See (host.Host).Connect for more information.
//
// RoutedHost's Connect differs in that if the host has no addresses for a
// given peer, it will use its routing system to try to find some.
func (rh *RoutedHost) Connect(ctx context.Context, pi peer.PeerInfo) error {
	// first, check if we're already connected.
	if len(rh.Network().ConnsToPeer(pi.ID)) > 0 {
		return nil
	}

	// if we were given some addresses, keep + use them.
	if len(pi.Addrs) > 0 {
		rh.Peerstore().AddAddrs(pi.ID, pi.Addrs, peer.TempAddrTTL)
	}

	// Check if we have some addresses in our recent memory.
	addrs := rh.Peerstore().Addrs(pi.ID)
	if len(addrs) < 1 {

		// no addrs? find some with the routing system.
		pi2, err := rh.route.FindPeer(ctx, pi.ID)
		if err != nil {
			return err // couldnt find any :(
		}
		if pi2.ID != pi.ID {
			err = fmt.Errorf("routing failure: provided addrs for different peer")
			logRoutingErrDifferentPeers(ctx, pi.ID, pi2.ID, err)
			return err
		}
		addrs = pi2.Addrs
	}

	// if we're here, we got some addrs. let's use our wrapped host to connect.
	pi.Addrs = addrs
	return rh.host.Connect(ctx, pi)
}

func logRoutingErrDifferentPeers(ctx context.Context, wanted, got peer.ID, err error) {
	lm := make(lgbl.DeferredMap)
	lm["error"] = err
	lm["wantedPeer"] = func() interface{} { return wanted.Pretty() }
	lm["gotPeer"] = func() interface{} { return got.Pretty() }
	log.Event(ctx, "routingError", lm)
}

func (rh *RoutedHost) ID() peer.ID {
	return rh.host.ID()
}
func (rh *RoutedHost) Peerstore() peer.Peerstore {
	return rh.host.Peerstore()
}
func (rh *RoutedHost) Addrs() []ma.Multiaddr {
	return rh.host.Addrs()
}
func (rh *RoutedHost) Network() inet.Network {
	return rh.host.Network()
}
func (rh *RoutedHost) Mux() *protocol.Mux {
	return rh.host.Mux()
}
func (rh *RoutedHost) SetStreamHandler(pid protocol.ID, handler inet.StreamHandler) {
	rh.host.SetStreamHandler(pid, handler)
}
func (rh *RoutedHost) NewStream(pid protocol.ID, p peer.ID) (inet.Stream, error) {
	return rh.host.NewStream(pid, p)
}
func (rh *RoutedHost) Close() error {
	// no need to close IpfsRouting. we dont own it.
	return rh.host.Close()
}
