package core

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	config "github.com/jbenet/go-ipfs/repo/config"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	math2 "github.com/jbenet/go-ipfs/thirdparty/math2"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	goprocess "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	periodicproc "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/periodic"
)

// ErrNotEnoughBootstrapPeers signals that we do not have enough bootstrap
// peers to bootstrap correctly.
var ErrNotEnoughBootstrapPeers = errors.New("not enough bootstrap peers to bootstrap")

const (
	// BootstrapPeriod governs the periodic interval at which the node will
	// attempt to bootstrap. The bootstrap process is not very expensive, so
	// this threshold can afford to be small (<=30s).
	BootstrapPeriod = 30 * time.Second

	// BootstrapPeerThreshold governs the node Bootstrap process. If the node
	// has less open connections than this number, it will open connections
	// to the bootstrap nodes. From there, the routing system should be able
	// to use the connections to the bootstrap nodes to connect to even more
	// peers. Routing systems like the IpfsDHT do so in their own Bootstrap
	// process, which issues random queries to find more peers.
	BootstrapPeerThreshold = 4

	// BootstrapConnectionTimeout determines how long to wait for a bootstrap
	// connection attempt before cancelling it.
	BootstrapConnectionTimeout time.Duration = BootstrapPeriod / 3
)

// nodeBootstrapper is a small object used to bootstrap an IpfsNode.
type nodeBootstrapper struct {
	node *IpfsNode
}

// TryToBootstrap starts IpfsNode bootstrapping. This function will run an
// initial bootstrapping phase before exiting: connect to several bootstrap
// nodes. This allows callers to call this function synchronously to:
// - check if an error occurrs (bootstrapping unsuccessful)
// - wait before starting services which require the node to be bootstrapped
//
// If bootstrapping initially fails, Bootstrap() will try again for a total of
// three times, before giving up completely. Note that in environments where a
// node may be initialized offline, as normal operation, BootstrapForever()
// should be used instead.
//
// Note: this function could be much cleaner if we were to relax the constraint
// that we want to exit **after** we have performed initial bootstrapping (and are
// thus connected to nodes). The constraint may not be that useful in practice.
// Consider cases when we initialize the node while disconnected from the internet.
// We don't want this launch to fail... want to continue launching the node, hoping
// that bootstrapping will work in the future if we get connected.
func (nb *nodeBootstrapper) TryToBootstrap(ctx context.Context, peers []peer.PeerInfo) error {
	n := nb.node

	// TODO what bootstrapping should happen if there is no DHT? i.e. we could
	// continue connecting to our bootstrap peers, but for what purpose? for now
	// simply exit without connecting to any of them. When we introduce another
	// routing system that uses bootstrap peers we can change this.
	dht, ok := n.Routing.(*dht.IpfsDHT)
	if !ok {
		return nil
	}

	for i := 0; i < 3; i++ {
		if err := bootstrapRound(ctx, n.PeerHost, dht, n.Peerstore, peers); err != nil {
			return err
		}
	}

	// at this point we have done at least one round of initial bootstrap.
	// we're ready to kick off dht bootstrapping.
	dbproc, err := dht.Bootstrap(ctx)
	if err != nil {
		return err
	}

	// kick off the node's periodic bootstrapping
	proc := periodicproc.Tick(BootstrapPeriod, func(worker goprocess.Process) {
		if err := bootstrapRound(ctx, n.PeerHost, dht, n.Peerstore, peers); err != nil {
			log.Error(err)
		}
	})

	// add dht bootstrap proc as a child, so it is closed automatically when we are.
	proc.AddChild(dbproc)

	// we were given a context. instead of returning proc for the caller
	// to manage, for now we just close the proc when context is done.
	go func() {
		<-ctx.Done()
		proc.Close()
	}()
	return nil
}

// BootstrapForever starts IpfsNode bootstrapping. Unlike TryToBootstrap(),
// BootstrapForever() will run indefinitely (until its context is cancelled).
// This is particularly useful for the daemon and other services, which may
// be started offline and will come online at a future date.
//
// TODO: check offline --to--> online case works well and doesn't hurt perf.
// We may still be dialing. We should check network config.
func (nb *nodeBootstrapper) BootstrapForever(ctx context.Context, peers []peer.PeerInfo) error {
	for {
		if err := nb.TryToBootstrap(ctx, peers); err == nil {
			return nil
		}
	}
}

func bootstrapRound(ctx context.Context,
	host host.Host,
	route *dht.IpfsDHT,
	peerstore peer.Peerstore,
	bootstrapPeers []peer.PeerInfo) error {

	ctx, _ = context.WithTimeout(ctx, BootstrapConnectionTimeout)

	// determine how many bootstrap connections to open
	connectedPeers := host.Network().Peers()
	if len(connectedPeers) >= BootstrapPeerThreshold {
		log.Event(ctx, "bootstrapSkip", host.ID())
		log.Debugf("%s core bootstrap skipped -- connected to %d (> %d) nodes",
			host.ID(), len(connectedPeers), BootstrapPeerThreshold)
		return nil
	}
	numCxnsToCreate := BootstrapPeerThreshold - len(connectedPeers)

	// filter out bootstrap nodes we are already connected to
	var notConnected []peer.PeerInfo
	for _, p := range bootstrapPeers {
		if host.Network().Connectedness(p.ID) != inet.Connected {
			notConnected = append(notConnected, p)
		}
	}

	// if connected to all bootstrap peer candidates, exit
	if len(notConnected) < 1 {
		log.Debugf("%s no more bootstrap peers to create %d connections", host.ID(), numCxnsToCreate)
		return ErrNotEnoughBootstrapPeers
	}

	// connect to a random susbset of bootstrap candidates
	var randomSubset = randomSubsetOfPeers(notConnected, numCxnsToCreate)
	log.Event(ctx, "bootstrapStart", host.ID())
	log.Debugf("%s bootstrapping to %d nodes: %s", host.ID(), numCxnsToCreate, randomSubset)
	if err := bootstrapConnect(ctx, peerstore, route, randomSubset); err != nil {
		log.Event(ctx, "bootstrapError", host.ID(), lgbl.Error(err))
		log.Errorf("%s bootstrap error: %s", host.ID(), err)
		return err
	}
	return nil
}

func bootstrapConnect(ctx context.Context,
	ps peer.Peerstore,
	route *dht.IpfsDHT,
	peers []peer.PeerInfo) error {
	if len(peers) < 1 {
		return ErrNotEnoughBootstrapPeers
	}

	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.

		wg.Add(1)
		go func(p peer.PeerInfo) {
			defer wg.Done()
			log.Event(ctx, "bootstrapDial", route.LocalPeer(), p.ID)
			log.Debugf("%s bootstrapping to %s", route.LocalPeer(), p.ID)

			ps.AddAddresses(p.ID, p.Addrs)
			err := route.Connect(ctx, p.ID)
			if err != nil {
				log.Event(ctx, "bootstrapFailed", p.ID)
				log.Errorf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			log.Event(ctx, "bootstrapSuccess", p.ID)
			log.Infof("bootstrapped with %v", p.ID)
		}(p)
	}
	wg.Wait()

	// our failure condition is when no connection attempt succeeded.
	// So drain the errs channel, counting the results.
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("failed to bootstrap. %s", err)
	}
	return nil
}

func toPeer(bootstrap config.BootstrapPeer) (p peer.PeerInfo, err error) {
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
