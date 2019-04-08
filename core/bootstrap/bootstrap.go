package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	logging "github.com/ipfs/go-log"
	"github.com/jbenet/goprocess"
	"github.com/jbenet/goprocess/context"
	"github.com/jbenet/goprocess/periodic"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-loggables"
	"github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-routing"
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
	BootstrapPeers func() []peerstore.PeerInfo
}

// DefaultBootstrapConfig specifies default sane parameters for bootstrapping.
var DefaultBootstrapConfig = BootstrapConfig{
	MinPeerThreshold:  4,
	Period:            30 * time.Second,
	ConnectionTimeout: (30 * time.Second) / 3, // Perod / 3
}

func BootstrapConfigWithPeers(pis []peerstore.PeerInfo) BootstrapConfig {
	cfg := DefaultBootstrapConfig
	cfg.BootstrapPeers = func() []peerstore.PeerInfo {
		return pis
	}
	return cfg
}

// Bootstrap kicks off IpfsNode bootstrapping. This function will periodically
// check the number of open connections and -- if there are too few -- initiate
// connections to well-known bootstrap peers. It also kicks off subsystem
// bootstrapping (i.e. routing).
func Bootstrap(id peer.ID, host host.Host, rt routing.IpfsRouting, cfg BootstrapConfig) (io.Closer, error) {

	// make a signal to wait for one bootstrap round to complete.
	doneWithRound := make(chan struct{})

	if len(cfg.BootstrapPeers()) == 0 {
		// We *need* to bootstrap but we have no bootstrap peers
		// configured *at all*, inform the user.
		log.Warning("no bootstrap nodes configured: go-ipfs may have difficulty connecting to the network")
	}

	// the periodic bootstrap function -- the connection supervisor
	periodic := func(worker goprocess.Process) {
		ctx := goprocessctx.OnClosingContext(worker)
		defer log.EventBegin(ctx, "periodicBootstrap", id).Done()

		if err := bootstrapRound(ctx, host, cfg); err != nil {
			log.Event(ctx, "bootstrapError", id, loggables.Error(err))
			log.Debugf("%s bootstrap error: %s", id, err)
		}

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
	return proc, nil
}

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
		log.Event(ctx, "bootstrapSkip", id)
		log.Debugf("%s core bootstrap skipped -- connected to %d (> %d) nodes",
			id, len(connected), cfg.MinPeerThreshold)
		return nil
	}
	numToDial := cfg.MinPeerThreshold - len(connected)

	// filter out bootstrap nodes we are already connected to
	var notConnected []peerstore.PeerInfo
	for _, p := range peers {
		if host.Network().Connectedness(p.ID) != net.Connected {
			notConnected = append(notConnected, p)
		}
	}

	// if connected to all bootstrap peer candidates, exit
	if len(notConnected) < 1 {
		log.Debugf("%s no more bootstrap peers to create %d connections", id, numToDial)
		return ErrNotEnoughBootstrapPeers
	}

	// connect to a random susbset of bootstrap candidates
	randSubset := randomSubsetOfPeers(notConnected, numToDial)

	defer log.EventBegin(ctx, "bootstrapStart", id).Done()
	log.Debugf("%s bootstrapping to %d nodes: %s", id, numToDial, randSubset)
	return bootstrapConnect(ctx, host, randSubset)
}

func bootstrapConnect(ctx context.Context, ph host.Host, peers []peerstore.PeerInfo) error {
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
		go func(p peerstore.PeerInfo) {
			defer wg.Done()
			defer log.EventBegin(ctx, "bootstrapDial", ph.ID(), p.ID).Done()
			log.Debugf("%s bootstrapping to %s", ph.ID(), p.ID)

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			if err := ph.Connect(ctx, p); err != nil {
				log.Event(ctx, "bootstrapDialFailed", p.ID)
				log.Debugf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			log.Event(ctx, "bootstrapDialSuccess", p.ID)
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

func randomSubsetOfPeers(in []peerstore.PeerInfo, max int) []peerstore.PeerInfo {
	if max > len(in) {
		max = len(in)
	}

	out := make([]peerstore.PeerInfo, max)
	for i, val := range rand.Perm(len(in))[:max] {
		out[i] = in[val]
	}
	return out
}

type Peers []config.BootstrapPeer

func (bpeers Peers) ToPeerInfos() []peerstore.PeerInfo {
	pinfos := make(map[peer.ID]*peerstore.PeerInfo)
	for _, bootstrap := range bpeers {
		pinfo, ok := pinfos[bootstrap.ID()]
		if !ok {
			pinfo = new(peerstore.PeerInfo)
			pinfos[bootstrap.ID()] = pinfo
			pinfo.ID = bootstrap.ID()
		}

		pinfo.Addrs = append(pinfo.Addrs, bootstrap.Transport())
	}

	var peers []peerstore.PeerInfo
	for _, pinfo := range pinfos {
		peers = append(peers, *pinfo)
	}

	return peers
}
