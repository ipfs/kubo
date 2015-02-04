package corerouting

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/p2p/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	supernode "github.com/jbenet/go-ipfs/routing/supernode"
	gcproxy "github.com/jbenet/go-ipfs/routing/supernode/proxy"
)

// NB: DHT option is included in the core to avoid 1) because it's a sane
// default and 2) to avoid a circular dependency (it needs to be referenced in
// the core if it's going to be the default)

var (
	errHostMissing      = errors.New("supernode routing client requires a Host component")
	errIdentityMissing  = errors.New("supernode routing server requires a peer ID identity")
	errPeerstoreMissing = errors.New("supernode routing server requires a peerstore")
	errServersMissing   = errors.New("supernode routing client requires at least 1 server peer")
)

// SupernodeServer returns a configuration for a routing server that stores
// routing records to the provided datastore. Only routing records are store in
// the datastore.
func SupernodeServer(recordSource datastore.ThreadSafeDatastore) core.RoutingOption {
	return func(ctx context.Context, node *core.IpfsNode) (routing.IpfsRouting, error) {
		if node.Peerstore == nil {
			return nil, errPeerstoreMissing
		}
		if node.PeerHost == nil {
			return nil, errHostMissing
		}
		if node.Identity == "" {
			return nil, errIdentityMissing
		}
		server, err := supernode.NewServer(recordSource, node.Peerstore, node.Identity)
		if err != nil {
			return nil, err
		}
		proxy := &gcproxy.Loopback{
			Handler: server,
			Local:   node.Identity,
		}
		node.PeerHost.SetStreamHandler(gcproxy.ProtocolSNR, proxy.HandleStream)
		return supernode.NewClient(proxy, node.PeerHost, node.Peerstore, node.Identity)
	}
}

// TODO doc
func SupernodeClient(remotes ...peer.PeerInfo) core.RoutingOption {
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

		var ids []peer.ID
		for _, info := range remotes {
			ids = append(ids, info.ID)
		}
		proxy := gcproxy.Standard(node.PeerHost, ids)
		node.PeerHost.SetStreamHandler(gcproxy.ProtocolSNR, proxy.HandleStream)
		return supernode.NewClient(proxy, node.PeerHost, node.Peerstore, node.Identity)
	}
}
