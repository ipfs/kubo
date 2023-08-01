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
	goprocessctx "github.com/jbenet/goprocess/context"
	periodicproc "github.com/jbenet/goprocess/periodic"
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

	// BackupBootstrapInterval governs the periodic interval at which the node will
	// attempt to save connected nodes to use as temporary bootstrap peers.
	BackupBootstrapInterval time.Duration

	// MaxBackupBootstrapSize controls the maximum number of peers we're saving
	// as backup bootstrap peers.
	MaxBackupBootstrapSize int

	saveBackupBootstrapPeers func(context.Context, []peer.AddrInfo)
	loadBackupBootstrapPeers func(context.Context) []peer.AddrInfo
}

// DefaultBootstrapConfig specifies default sane parameters for bootstrapping.
var DefaultBootstrapConfig = BootstrapConfig{
	MinPeerThreshold:        4,
	Period:                  30 * time.Second,
	ConnectionTimeout:       (30 * time.Second) / 3, // Perod / 3
	BackupBootstrapInterval: 1 * time.Hour,
	MaxBackupBootstrapSize:  20,
}

// BootstrapConfigWithPeers creates a default BootstrapConfig configured with
// the specified peers, and optional functions to load and save backup peers.
func BootstrapConfigWithPeers(pis []peer.AddrInfo, options ...func(*BootstrapConfig)) BootstrapConfig {
	cfg := DefaultBootstrapConfig
	cfg.BootstrapPeers = func() []peer.AddrInfo {
		return pis
	}
	for _, opt := range options {
		opt(&cfg)
	}
	return cfg
}

// WithBackupPeers configures functions to load and save backup bootstrap peers.
func WithBackupPeers(load func(context.Context) []peer.AddrInfo, save func(context.Context, []peer.AddrInfo)) func(*BootstrapConfig) {
	if save == nil && load != nil || save != nil && load == nil {
		panic("both load and save backup bootstrap peers functions must be defined")
	}
	return func(cfg *BootstrapConfig) {
		cfg.loadBackupBootstrapPeers = load
		cfg.saveBackupBootstrapPeers = save
	}
}

// BackupPeers returns the load and save backup peers functions.
func (cfg *BootstrapConfig) BackupPeers() (func(context.Context) []peer.AddrInfo, func(context.Context, []peer.AddrInfo)) {
	return cfg.loadBackupBootstrapPeers, cfg.saveBackupBootstrapPeers
}

// SetBackupPeers sets the load and save backup peers functions.
func (cfg *BootstrapConfig) SetBackupPeers(load func(context.Context) []peer.AddrInfo, save func(context.Context, []peer.AddrInfo)) {
	opt := WithBackupPeers(load, save)
	opt(cfg)
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

	// If loadBackupBootstrapPeers is not nil then saveBackupBootstrapPeers
	// must also not be nil.
	if cfg.loadBackupBootstrapPeers != nil {
		startSavePeersAsTemporaryBootstrapProc(cfg, host, proc)
	}

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
	savePeersProc := periodicproc.Tick(cfg.BackupBootstrapInterval, savePeersFn)

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
	// Randomize the list of connected peers, we don't prioritize anyone.
	connectedPeers := randomizeList(host.Network().Peers())

	bootstrapPeers := cfg.BootstrapPeers()
	backupPeers := make([]peer.AddrInfo, 0, cfg.MaxBackupBootstrapSize)

	// Choose peers to save and filter out the ones that are already bootstrap nodes.
	for _, p := range connectedPeers {
		found := false
		for _, bootstrapPeer := range bootstrapPeers {
			if p == bootstrapPeer.ID {
				found = true
				break
			}
		}
		if !found {
			backupPeers = append(backupPeers, peer.AddrInfo{
				ID:    p,
				Addrs: host.Network().Peerstore().Addrs(p),
			})
		}

		if len(backupPeers) >= cfg.MaxBackupBootstrapSize {
			break
		}
	}

	// If we didn't reach the target number use previously stored connected peers.
	if len(backupPeers) < cfg.MaxBackupBootstrapSize {
		oldSavedPeers := cfg.loadBackupBootstrapPeers(ctx)
		log.Debugf("missing %d peers to reach backup bootstrap target of %d, trying from previous list of %d saved peers",
			cfg.MaxBackupBootstrapSize-len(backupPeers), cfg.MaxBackupBootstrapSize, len(oldSavedPeers))

		// Add some of the old saved peers. Ensure we don't duplicate them.
		for _, p := range oldSavedPeers {
			found := false
			for _, sp := range backupPeers {
				if p.ID == sp.ID {
					found = true
					break
				}
			}

			if !found {
				backupPeers = append(backupPeers, p)
			}

			if len(backupPeers) >= cfg.MaxBackupBootstrapSize {
				break
			}
		}
	}

	cfg.saveBackupBootstrapPeers(ctx, backupPeers)
	log.Debugf("saved %d peers (of %d target) as bootstrap backup in the config", len(backupPeers), cfg.MaxBackupBootstrapSize)
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

	if cfg.loadBackupBootstrapPeers == nil {
		log.Debugf("not enough bootstrap peers to fill the remaining target of %d connections", numToDial)
		return ErrNotEnoughBootstrapPeers
	}

	log.Debugf("not enough bootstrap peers to fill the remaining target of %d connections, trying backup list", numToDial)

	tempBootstrapPeers := cfg.loadBackupBootstrapPeers(ctx)
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
	peers := randomizeList(availablePeers)

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
				ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			}

			log.Infof("bootstrapped with %v", p.ID)
			atomic.AddUint64(&connected, 1)
		}(p)
	}
	wg.Wait()

	return connected
}

func randomizeList[T any](in []T) []T {
	out := make([]T, len(in))
	for i, val := range rand.Perm(len(in)) {
		out[i] = in[val]
	}
	return out
}
