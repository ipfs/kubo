package bootstrap

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/jbenet/goprocess"
	"github.com/jbenet/goprocess/context"
	"github.com/jbenet/goprocess/periodic"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
)

var log = logging.Logger("bootstrap")

// ErrNotEnoughBootstrapPeers signals that we do not have enough bootstrap
// peers to bootstrap correctly.
var ErrNotEnoughBootstrapPeers = errors.New("not enough bootstrap peers to bootstrap")

// BootstrapConfig specifies parameters used in an IpfsNode's network
// bootstrapping process.
type BootstrapConfig struct {
	// MinPeerThreshold governs whether to bootstrap more connections. If the
	// node has less open connections than this number, it will open connections
	// to the bootstrap nodes. From there, the routing system should be able
	// to use the connections to the bootstrap nodes to connect to even more
	// peers. Routing systems like the IpfsDHT do so in their own Bootstrap
	// process, which issues random queries to find more peers.
	MinPeerThreshold int

	// Period governs the periodic interval at which the node will
	// attempt to bootstrap. The bootstrap process is not very expensive, so
	// this threshold can afford to be small (<=30s).
	Period time.Duration

	// ConnectionTimeout determines how long to wait for a bootstrap
	// connection attempt before cancelling it.
	ConnectionTimeout time.Duration

	// BootstrapPeers is a function that returns a set of bootstrap peers
	// for the bootstrap process to use. This makes it possible for clients
	// to control the peers the process uses at any moment.
	BootstrapPeers func() []peer.AddrInfo

	// FIXME(BLOCKING): Review names, default values and doc.
	// SavePeersPeriod governs the periodic interval at which the node will
	// attempt to save connected nodes to use as temporary bootstrap peers.
	SavePeersPeriod time.Duration
	// SaveConnectedPeersRatio controls the number peers we're saving compared
	// to the target MinPeerThreshold. For example, if  MinPeerThreshold is 4,
	// and we have a ratio of 5 we will save 20 connected peers.
	// Note: one peer can have many addresses under its ID, so saving a peer
	// might translate to more than one line in the config (following the above
	// example that means TempBootstrapPeers may have more than 20 lines, but
	// all those lines will be addresses of at most 20 peers).
	SaveConnectedPeersRatio   int
	SaveTempPeersForBootstrap func(context.Context, []peer.AddrInfo)
	LoadTempPeersForBootstrap func(context.Context) []peer.AddrInfo
}

// DefaultBootstrapConfig specifies default sane parameters for bootstrapping.
var DefaultBootstrapConfig = BootstrapConfig{
	MinPeerThreshold:  4,
	Period:            30 * time.Second,
	ConnectionTimeout: (30 * time.Second) / 3, // Perod / 3
	// FIXME(BLOKING): Review this number. We're making it ridiculously small
	//  only for testing purposes, but this is saving the peers to the config
	//  file every time so should not be run frequently. (Original proposal 24
	//  hours.)
	SavePeersPeriod:         10 * time.Second,
	SaveConnectedPeersRatio: 2,
}

func BootstrapConfigWithPeers(pis []peer.AddrInfo) BootstrapConfig {
	cfg := DefaultBootstrapConfig
	cfg.BootstrapPeers = func() []peer.AddrInfo {
		return pis
	}
	return cfg
}

// Bootstrap kicks off IpfsNode bootstrapping. This function will periodically
// check the number of open connections and -- if there are too few -- initiate
// connections to well-known bootstrap peers. It also kicks off subsystem
// bootstrapping (i.e. routing).
func Bootstrap(id peer.ID, host host.Host, rt routing.Routing, cfg BootstrapConfig) (io.Closer, error) {

	// make a signal to wait for one bootstrap round to complete.
	doneWithRound := make(chan struct{})

	if len(cfg.BootstrapPeers()) == 0 {
		// We *need* to bootstrap but we have no bootstrap peers
		// configured *at all*, inform the user.
		log.Warn("no bootstrap nodes configured: go-ipfs may have difficulty connecting to the network")
	}

	// the periodic bootstrap function -- the connection supervisor
	periodic := func(worker goprocess.Process) {
		ctx := goprocessctx.OnClosingContext(worker)

		if err := bootstrapRound(ctx, host, cfg); err != nil {
			log.Debugf("%s bootstrap error: %s", id, err)
		}

		// Exit the first call (triggered independently by `proc.Go`, not `Tick`)
		// only after being done with the *single* Routing.Bootstrap call. Following
		// periodic calls (`Tick`) will not block on this.
		<-doneWithRound
	}

	// kick off the node's periodic bootstrapping
	proc := periodicproc.Tick(cfg.Period, periodic)
	proc.Go(periodic) // run one right now.

	// kick off Routing.Bootstrap
	if rt != nil {
		ctx := goprocessctx.OnClosingContext(proc)
		if err := rt.Bootstrap(ctx); err != nil {
			proc.Close()
			return nil, err
		}
	}

	doneWithRound <- struct{}{}
	close(doneWithRound) // it no longer blocks periodic

	startSavePeersAsTemporaryBootstrapProc(cfg, host, proc)

	return proc, nil
}

// Aside of the main bootstrap process we also run a secondary one that saves
// connected peers as a backup measure if we can't connect to the official
// bootstrap ones. These peers will serve as *temporary* bootstrap nodes.
func startSavePeersAsTemporaryBootstrapProc(cfg BootstrapConfig, host host.Host, bootstrapProc goprocess.Process) {

	savePeersFn := func(worker goprocess.Process) {
		ctx := goprocessctx.OnClosingContext(worker)

		if err := saveConnectedPeersAsTemporaryBootstrap(ctx, host, cfg); err != nil {
			log.Debugf("saveConnectedPeersAsTemporaryBootstrap error: %s", err)
		}
	}
	savePeersProc := periodicproc.Tick(cfg.SavePeersPeriod, savePeersFn)
	// When the main bootstrap process ends also terminate the 'save connected
	// peers' ones. Coupling the two seems the easiest way to handle this backup
	// process without additional complexity.
	go func() {
		<-bootstrapProc.Closing()
		savePeersProc.Close()
	}()
	// Run the first round now (after the first bootstrap process has finished)
	// as the SavePeersPeriod can be much longer than bootstrap.
	savePeersProc.Go(savePeersFn)
}

func saveConnectedPeersAsTemporaryBootstrap(ctx context.Context, host host.Host, cfg BootstrapConfig) error {
	allConnectedPeers := host.Network().Peers()
	// Randomize the list of connected peers, we don't prioritize anyone.
	// FIXME: Maybe use randomizeAddressList if we change from []peer.ID to
	//  []peer.AddrInfo earlier in the logic.
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allConnectedPeers),
		func(i, j int) {
			allConnectedPeers[i], allConnectedPeers[j] = allConnectedPeers[j], allConnectedPeers[i]
		})

	saveNumber := cfg.SaveConnectedPeersRatio * cfg.MinPeerThreshold
	savedPeers := make([]peer.AddrInfo, 0, saveNumber)

	// Save peers from the connected list that aren't bootstrap ones.
	bootsrapPeers := cfg.BootstrapPeers()
OUTER:
	for _, p := range allConnectedPeers {
		for _, bootstrapPeer := range bootsrapPeers {
			if p == bootstrapPeer.ID {
				continue OUTER
			}
		}
		savedPeers = append(savedPeers,
			peer.AddrInfo{ID: p, Addrs: host.Network().Peerstore().Addrs(p)})
		if len(savedPeers) >= saveNumber {
			break
		}
	}

	// If we didn't reach the target number use previously stored connected peers.
	if len(savedPeers) < saveNumber {
		oldSavedPeers := cfg.LoadTempPeersForBootstrap(ctx)
		log.Debugf("missing %d peers to reach backup bootstrap target of %d, trying from previous list of %d saved peers",
			saveNumber-len(savedPeers), saveNumber, len(oldSavedPeers))
		for _, p := range oldSavedPeers {
			savedPeers = append(savedPeers, p)
			if len(savedPeers) >= saveNumber {
				break
			}
		}
	}

	cfg.SaveTempPeersForBootstrap(ctx, savedPeers)
	log.Debugf("saved %d connected peers (of %d target) as bootstrap backup in the config", len(savedPeers), saveNumber)
	return nil
}

// Connect to as many peers needed to reach the BootstrapConfig.MinPeerThreshold.
// Peers can be original bootstrap or temporary ones (drawn from a list of
// persisted previously connected peers).
func bootstrapRound(ctx context.Context, host host.Host, cfg BootstrapConfig) error {

	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionTimeout)
	defer cancel()
	id := host.ID()

	// get bootstrap peers from config. retrieving them here makes
	// sure we remain observant of changes to client configuration.
	peers := cfg.BootstrapPeers()
	// determine how many bootstrap connections to open
	connected := host.Network().Peers()
	if len(connected) >= cfg.MinPeerThreshold {
		log.Debugf("%s core bootstrap skipped -- connected to %d (> %d) nodes",
			id, len(connected), cfg.MinPeerThreshold)
		return nil
	}
	numToDial := cfg.MinPeerThreshold - len(connected) // numToDial > 0

	if len(peers) > 0 {
		numToDial -= int(peersConnect(ctx, host, peers, numToDial, true))
		if numToDial <= 0 {
			return nil
		}
	}

	log.Debugf("not enough bootstrap peers to fill the remaining target of %d connections, trying backup list", numToDial)

	tempBootstrapPeers := cfg.LoadTempPeersForBootstrap(ctx)
	if len(tempBootstrapPeers) > 0 {
		numToDial -= int(peersConnect(ctx, host, tempBootstrapPeers, numToDial, false))
		if numToDial <= 0 {
			return nil
		}
	}

	log.Debugf("tried both original bootstrap peers and temporary ones but still missing target of %d connections", numToDial)

	return ErrNotEnoughBootstrapPeers
}

// Attempt to make `needed` connections from the `availablePeers` list. Mark
// peers as either `permanent` or temporary when adding them to the Peerstore.
// Return the number of connections completed. We eagerly over-connect in parallel,
// so we might connect to more than needed.
// (We spawn as many routines and attempt connections as the number of availablePeers,
// but this list comes from restricted sets of original or temporary bootstrap
// nodes which will keep it under a sane value.)
func peersConnect(ctx context.Context, ph host.Host, availablePeers []peer.AddrInfo, needed int, permanent bool) uint64 {
	peers := randomizeAddressList(availablePeers)

	// Monitor the number of connections and stop if we reach the target.
	var connected uint64
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				if int(atomic.LoadUint64(&connected)) >= needed {
					cancel()
					return
				}
			}
		}
	}()

	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.

		if int(atomic.LoadUint64(&connected)) >= needed {
			cancel()
			break
		}

		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()

			// Skip addresses belonging to a peer we're already connected to.
			// (Not a guarantee but a best-effort policy.)
			if ph.Network().Connectedness(p.ID) == network.Connected {
				return
			}
			log.Debugf("%s bootstrapping to %s", ph.ID(), p.ID)

			if err := ph.Connect(ctx, p); err != nil {
				if ctx.Err() != context.Canceled {
					log.Debugf("failed to bootstrap with %v: %s", p.ID, err)
				}
				return
			}
			if permanent {
				// We're connecting to an original bootstrap peer, mark it as
				// a permanent address (Connect will register it as TempAddrTTL).
				// FIXME(BLOCKING): From the code it seems this will overwrite the
				//  temporary TTL from Connect: need confirmation from libp2p folks.
				//  Registering it *after* the connect give less chances of registering
				//  many addresses we won't be using in case we already reached the
				//  target and the context has already been cancelled. (This applies
				//  only to the very restricted list of original bootstrap nodes so
				//  this issue is not critical.)
				ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			}

			log.Infof("bootstrapped with %v", p.ID)
			atomic.AddUint64(&connected, 1)
		}(p)
	}
	wg.Wait()

	return connected
}

func randomizeAddressList(in []peer.AddrInfo) []peer.AddrInfo {
	out := make([]peer.AddrInfo, len(in))
	for i, val := range rand.Perm(len(in)) {
		out[i] = in[val]
	}
	return out
}
