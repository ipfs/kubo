package core

import (
	"math/rand"
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	config "github.com/jbenet/go-ipfs/config"
	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	math2 "github.com/jbenet/go-ipfs/util/math2"
)

const (
	period                          = 30 * time.Second // how often to check connection status
	connectiontimeout time.Duration = period / 3       // duration to wait when attempting to connect
	recoveryThreshold               = 4                // attempt to bootstrap if connection count falls below this value
)

func superviseConnections(parent context.Context,
	n inet.Network,
	route *dht.IpfsDHT, // TODO depend on abstract interface for testing purposes
	store peer.Peerstore,
	peers []*config.BootstrapPeer) error {

	for {
		ctx, _ := context.WithTimeout(parent, connectiontimeout)
		// TODO get config from disk so |peers| always reflects the latest
		// information
		if err := bootstrap(ctx, n, route, store, peers); err != nil {
			log.Error(err)
		}
		select {
		case <-parent.Done():
			return parent.Err()
		case <-time.Tick(period):
		}
	}
	return nil
}

func bootstrap(ctx context.Context,
	n inet.Network,
	r *dht.IpfsDHT,
	ps peer.Peerstore,
	boots []*config.BootstrapPeer) error {

	if len(n.GetConnections()) >= recoveryThreshold {
		return nil
	}
	numCxnsToCreate := recoveryThreshold - len(n.GetConnections())

	var bootstrapPeers []peer.Peer
	for _, bootstrap := range boots {
		p, err := toPeer(ps, bootstrap)
		if err != nil {
			return err
		}
		bootstrapPeers = append(bootstrapPeers, p)
	}

	var notConnected []peer.Peer
	for _, p := range bootstrapPeers {
		if !n.IsConnected(p) {
			notConnected = append(notConnected, p)
		}
	}

	var randomSubset = randomSubsetOfPeers(notConnected, numCxnsToCreate)
	if err := connect(ctx, r, randomSubset); err != nil {
		return err
	}
	return nil
}

func connect(ctx context.Context, r *dht.IpfsDHT, peers []peer.Peer) error {
	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.

		wg.Add(1)
		go func(p peer.Peer) {
			defer wg.Done()
			err := r.Connect(ctx, p)
			if err != nil {
				log.Event(ctx, "bootstrapFailed", p)
				log.Criticalf("failed to bootstrap with %v", p)
				return
			}
			log.Event(ctx, "bootstrapSuccess", p)
			log.Infof("bootstrapped with %v", p)
		}(p)
	}
	wg.Wait()
	return nil
}

func toPeer(ps peer.Peerstore, bootstrap *config.BootstrapPeer) (peer.Peer, error) {
	id, err := peer.DecodePrettyID(bootstrap.PeerID)
	if err != nil {
		return nil, err
	}
	p, err := ps.FindOrCreate(id)
	if err != nil {
		return nil, err
	}
	maddr, err := ma.NewMultiaddr(bootstrap.Address)
	if err != nil {
		return nil, err
	}
	p.AddAddress(maddr)
	return p, nil
}

func randomSubsetOfPeers(in []peer.Peer, max int) []peer.Peer {
	n := math2.IntMin(max, len(in))
	var out []peer.Peer
	for _, val := range rand.Perm(n) {
		out = append(out, in[val])
	}
	return out
}
