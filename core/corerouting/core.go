package corerouting

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/p2p/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	grandcentral "github.com/jbenet/go-ipfs/routing/grandcentral"
	gcproxy "github.com/jbenet/go-ipfs/routing/grandcentral/proxy"
)

// NB: DHT option is included in the core to avoid 1) because it's a sane
// default and 2) to avoid a circular dependency (it needs to be referenced in
// the core if it's going to be the default)

var (
	errHostMissing      = errors.New("grandcentral client requires a Host component")
	errIdentityMissing  = errors.New("grandcentral server requires a peer ID identity")
	errPeerstoreMissing = errors.New("grandcentral server requires a peerstore")
	errServersMissing   = errors.New("grandcentral client requires at least 1 server peer")
)

// TODO doc
func GrandCentralServer(recordSource datastore.ThreadSafeDatastore) core.RoutingOption {
	return func(ctx context.Context, node *core.IpfsNode) (routing.IpfsRouting, error) {
		if node.Peerstore == nil {
			return nil, errPeerstoreMissing
		}
		if node.Identity == "" {
			return nil, errIdentityMissing
		}
		server, err := grandcentral.NewServer(recordSource, node.Peerstore, node.Identity)
		if err != nil {
			return nil, err
		}
		proxy := &gcproxy.Loopback{
			Handler: server,
			Local:   node.Identity,
		}
		return grandcentral.NewClient(proxy, node.Peerstore, node.Identity)
	}
}

// TODO doc
func GrandCentralClient(remotes ...peer.PeerInfo) core.RoutingOption {
	return func(ctx context.Context, node *core.IpfsNode) (routing.IpfsRouting, error) {
		if len(remotes) < 1 {
			return nil, errServersMissing
		}
		if node.PeerHost == nil {
			return nil, errHostMissing
		}
		if node.Identity == "" {
			return nil, errIdentityMissing
		}
		if node.Peerstore == nil {
			return nil, errors.New("need peerstore")
		}

		// TODO move to bootstrap method
		for _, info := range remotes {
			if err := node.PeerHost.Connect(ctx, info); err != nil {
				return nil, err // TODO
			}
		}

		// TODO right now, I think this has a hidden dependency on the
		// bootstrap peers provided to the core.Node. Careful...

		var ids []peer.ID
		for _, info := range remotes {
			ids = append(ids, info.ID)
		}
		proxy := gcproxy.Standard(node.PeerHost, ids)
		return grandcentral.NewClient(proxy, node.Peerstore, node.Identity)
	}
}
