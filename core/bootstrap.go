package core

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	config "github.com/jbenet/go-ipfs/config"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
	math2 "github.com/jbenet/go-ipfs/util/math2"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
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

	connectedPeers := n.Peers()
	if len(connectedPeers) >= recoveryThreshold {
		log.Event(ctx, "bootstrapSkip", n.LocalPeer())
		log.Debugf("%s bootstrap skipped -- connected to %d (> %d) nodes",
			n.LocalPeer(), len(connectedPeers), recoveryThreshold)

		return nil
	}
	numCxnsToCreate := recoveryThreshold - len(connectedPeers)

	log.Event(ctx, "bootstrapStart", n.LocalPeer())
	log.Debugf("%s bootstrapping to %d more nodes", n.LocalPeer(), numCxnsToCreate)

	var bootstrapPeers []peer.PeerInfo
	for _, bootstrap := range boots {
		p, err := toPeer(bootstrap)
		if err != nil {
			log.Event(ctx, "bootstrapError", n.LocalPeer(), lgbl.Error(err))
			log.Errorf("%s bootstrap error: %s", n.LocalPeer(), err)
			return err
		}
		bootstrapPeers = append(bootstrapPeers, p)
	}

	var notConnected []peer.PeerInfo
	for _, p := range bootstrapPeers {
		if n.Connectedness(p.ID) != inet.Connected {
			notConnected = append(notConnected, p)
		}
	}

	if len(notConnected) < 1 {
		s := "must bootstrap to %d more nodes, but already connected to all candidates"
		err := fmt.Errorf(s, numCxnsToCreate)
		log.Event(ctx, "bootstrapError", n.LocalPeer(), lgbl.Error(err))
		log.Errorf("%s bootstrap error: %s", n.LocalPeer(), err)
		return err
	}

	var randomSubset = randomSubsetOfPeers(notConnected, numCxnsToCreate)

	log.Debugf("%s bootstrapping to %d nodes: %s", n.LocalPeer(), numCxnsToCreate, randomSubset)
	if err := connect(ctx, ps, r, randomSubset); err != nil {
		log.Event(ctx, "bootstrapError", n.LocalPeer(), lgbl.Error(err))
		log.Errorf("%s bootstrap error: %s", n.LocalPeer(), err)
		return err
	}
	return nil
}

func connect(ctx context.Context, ps peer.Peerstore, r *dht.IpfsDHT, peers []peer.PeerInfo) error {
	if len(peers) < 1 {
		return errors.New("bootstrap set empty")
	}

	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.

		wg.Add(1)
		go func(p peer.PeerInfo) {
			defer wg.Done()
			log.Event(ctx, "bootstrapDial", r.LocalPeer(), p.ID)
			log.Debugf("%s bootstrapping to %s", r.LocalPeer(), p.ID)

			ps.AddAddresses(p.ID, p.Addrs)
			err := r.Connect(ctx, p.ID)
			if err != nil {
				log.Event(ctx, "bootstrapFailed", p.ID)
				log.Criticalf("failed to bootstrap with %v", p.ID)
				return
			}
			log.Event(ctx, "bootstrapSuccess", p.ID)
			log.Infof("bootstrapped with %v", p.ID)
		}(p)
	}
	wg.Wait()
	return nil
}

func toPeer(bootstrap *config.BootstrapPeer) (p peer.PeerInfo, err error) {
	id, err := peer.IDB58Decode(bootstrap.PeerID)
	if err != nil {
		return
	}
	maddr, err := ma.NewMultiaddr(bootstrap.Address)
	if err != nil {
		return
	}
	p = peer.PeerInfo{
		ID:    id,
		Addrs: []ma.Multiaddr{maddr},
	}
	return
}

func randomSubsetOfPeers(in []peer.PeerInfo, max int) []peer.PeerInfo {
	n := math2.IntMin(max, len(in))
	var out []peer.PeerInfo
	for _, val := range rand.Perm(n) {
		out = append(out, in[val])
	}
	return out
}
