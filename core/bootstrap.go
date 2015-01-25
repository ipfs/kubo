package core

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	procctx "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	periodicproc "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/periodic"
)

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
	BootstrapPeers func() []peer.PeerInfo
}

// DefaultBootstrapConfig specifies default sane parameters for bootstrapping.
var DefaultBootstrapConfig = BootstrapConfig{
	MinPeerThreshold:  4,
	Period:            30 * time.Second,
	ConnectionTimeout: (30 * time.Second) / 3, // Perod / 3
}

func BootstrapConfigWithPeers(pis []peer.PeerInfo) BootstrapConfig {
	cfg := DefaultBootstrapConfig
	cfg.BootstrapPeers = func() []peer.PeerInfo {
		return pis
	}
	return cfg
}

// Bootstrap kicks off IpfsNode bootstrapping. This function will periodically
// check the number of open connections and -- if there are too few -- initiate
// connections to well-known bootstrap peers. It also kicks off subsystem
// bootstrapping (i.e. routing).
func Bootstrap(n *IpfsNode, cfg BootstrapConfig) (io.Closer, error) {

	// TODO what bootstrapping should happen if there is no DHT? i.e. we could
	// continue connecting to our bootstrap peers, but for what purpose? for now
	// simply exit without connecting to any of them. When we introduce another
	// routing system that uses bootstrap peers we can change this.
	thedht, ok := n.Routing.(*dht.IpfsDHT)
	if !ok {
		return ioutil.NopCloser(nil), nil
	}

	// the periodic bootstrap function -- the connection supervisor
	periodic := func(worker goprocess.Process) {
		ctx := procctx.WithProcessClosing(context.Background(), worker)
		defer log.EventBegin(ctx, "periodicBootstrap", n.Identity).Done()

		if err := bootstrapRound(ctx, n.PeerHost, thedht, n.Peerstore, cfg); err != nil {
			log.Event(ctx, "bootstrapError", n.Identity, lgbl.Error(err))
			log.Debugf("%s bootstrap error: %s", n.Identity, err)
		}
	}

	// kick off the node's periodic bootstrapping
	proc := periodicproc.Tick(cfg.Period, periodic)
	proc.Go(periodic) // run one right now.

	// kick off dht bootstrapping.
	dbproc, err := thedht.Bootstrap(dht.DefaultBootstrapConfig)
	if err != nil {
		proc.Close()
		return nil, err
	}

	// add dht bootstrap proc as a child, so it is closed automatically when we are.
	proc.AddChild(dbproc)
	return proc, nil
}

func bootstrapRound(ctx context.Context,
	host host.Host,
	route *dht.IpfsDHT,
	peerstore peer.Peerstore,
	cfg BootstrapConfig) error {

	ctx, _ = context.WithTimeout(ctx, cfg.ConnectionTimeout)
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
	var notConnected []peer.PeerInfo
	for _, p := range peers {
		if host.Network().Connectedness(p.ID) != inet.Connected {
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
	if err := bootstrapConnect(ctx, peerstore, route, randSubset); err != nil {
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
			defer log.EventBegin(ctx, "bootstrapDial", route.LocalPeer(), p.ID).Done()
			log.Debugf("%s bootstrapping to %s", route.LocalPeer(), p.ID)

			ps.AddAddresses(p.ID, p.Addrs)
			err := route.Connect(ctx, p.ID)
			if err != nil {
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

func toPeerInfos(bpeers []config.BootstrapPeer) ([]peer.PeerInfo, error) {
	var peers []peer.PeerInfo
	for _, bootstrap := range bpeers {
		p, err := toPeerInfo(bootstrap)
		if err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, nil
}

func toPeerInfo(bootstrap config.BootstrapPeer) (p peer.PeerInfo, err error) {
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
