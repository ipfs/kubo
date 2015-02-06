package core

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	debugerror "github.com/jbenet/go-ipfs/util/debugerror"

	p2phost "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	routing "github.com/jbenet/go-ipfs/routing"

	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	offline "github.com/jbenet/go-ipfs/exchange/offline"

	repo "github.com/jbenet/go-ipfs/repo"
)

type ConfigOption func(ctx context.Context) (*IpfsNode, error)
type NetworkSetupOption func(context.Context, peer.ID, peer.Peerstore) (p2phost.Host, error)

// Offline returns an IpfsNode ConfigOption for an instance in offline mode
func Offline(r repo.Repo) ConfigOption {
	return standardWithRouting(r, false, nil)
}

// OnlineWithRouting returns an IpfsNode ConfigOption for an instance in offline mode
// but with the option of specifying the routing system to be used.
func OnlineWithRouting(r repo.Repo, router routing.IpfsRouting) ConfigOption {
	if router == nil {
		panic("router required")
	}
	return standardWithRouting(r, true, router)
}

// Offline returns an IpfsNode ConfigOption for an instance in online mode
func Online(r repo.Repo) ConfigOption {
	return standardWithRouting(r, true, nil)
}

// TODO refactor so maybeRouter isn't special-cased in this way
func standardWithRouting(r repo.Repo, online bool, maybeRouter routing.IpfsRouting) ConfigOption {
	return func(ctx context.Context) (n *IpfsNode, err error) {
		// FIXME perform node construction in the main constructor so it isn't
		// necessary to perform this teardown in this scope.
		success := false
		defer func() {
			if !success && n != nil {
				n.teardown()
			}
		}()

		// TODO move as much of node initialization as possible into
		// NewIPFSNode. The larger these config options are, the harder it is
		// to test all node construction code paths.

		if r == nil {
			return nil, debugerror.Errorf("repo required")
		}
		n = &IpfsNode{
			mode: func() mode {
				if online {
					return onlineMode
				}
				return offlineMode
			}(),
			Repo: r,
		}

		// setup Peerstore
		n.Peerstore = peer.NewPeerstore()

		// setup local peer ID (private key is loaded in online setup)
		if err := n.loadID(); err != nil {
			return nil, err
		}

		n.Blockstore, err = bstore.WriteCached(bstore.NewBlockstore(n.Repo.Datastore()), kSizeBlockstoreWriteCache)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}

		if online {
			if err := n.startOnlineServices(ctx, maybeRouter, constructPeerHost); err != nil {
				return nil, err
			}
		} else {
			n.Exchange = offline.Exchange(n.Blockstore)
		}

		success = true
		return n, nil
	}
}
