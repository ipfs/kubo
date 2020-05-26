package peering

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Seed the random number generator.
//
// We don't need good randomness, but we do need randomness.
const (
	// maxBackoff is the maximum time between reconnect attempts.
	maxBackoff = 10 * time.Minute
	// The backoff will be cut off when we get within 10% of the actual max.
	// If we go over the max, we'll adjust the delay down to a random value
	// between 90-100% of the max backoff.
	maxBackoffJitter = 10 // %
	connmgrTag       = "ipfs-peering"
	// This needs to be sufficient to prevent two sides from simultaneously
	// dialing.
	initialDelay = 5 * time.Second
)

var logger = log.Logger("peering")

type state int

const (
	stateInit state = iota
	stateRunning
	stateStopped
)

// peerHandler keeps track of all state related to a specific "peering" peer.
type peerHandler struct {
	peer   peer.ID
	host   host.Host
	ctx    context.Context
	cancel context.CancelFunc

	mu             sync.Mutex
	addrs          []multiaddr.Multiaddr
	reconnectTimer *time.Timer

	nextDelay time.Duration
}

// setAddrs sets the addresses for this peer.
func (ph *peerHandler) setAddrs(addrs []multiaddr.Multiaddr) {
	// Not strictly necessary, but it helps to not trust the calling code.
	addrCopy := make([]multiaddr.Multiaddr, len(addrs))
	copy(addrCopy, addrs)

	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.addrs = addrCopy
}

// getAddrs returns a shared slice of addresses for this peer. Do not modify.
func (ph *peerHandler) getAddrs() []multiaddr.Multiaddr {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	return ph.addrs
}

// stop permanently stops the peer handler.
func (ph *peerHandler) stop() {
	ph.cancel()

	ph.mu.Lock()
	defer ph.mu.Unlock()
	if ph.reconnectTimer != nil {
		ph.reconnectTimer.Stop()
		ph.reconnectTimer = nil
	}
}

func (ph *peerHandler) nextBackoff() time.Duration {
	if ph.nextDelay < maxBackoff {
		ph.nextDelay += ph.nextDelay/2 + time.Duration(rand.Int63n(int64(ph.nextDelay)))
	}

	// If we've gone over the max backoff, reduce it under the max.
	if ph.nextDelay > maxBackoff {
		ph.nextDelay = maxBackoff
		// randomize the backoff a bit (10%).
		ph.nextDelay -= time.Duration(rand.Int63n(int64(maxBackoff) * maxBackoffJitter / 100))
	}

	return ph.nextDelay
}

func (ph *peerHandler) reconnect() {
	// Try connecting
	addrs := ph.getAddrs()
	logger.Debugw("reconnecting", "peer", ph.peer, "addrs", addrs)

	err := ph.host.Connect(ph.ctx, peer.AddrInfo{ID: ph.peer, Addrs: addrs})
	if err != nil {
		logger.Debugw("failed to reconnect", "peer", ph.peer, "error", err)
		// Ok, we failed. Extend the timeout.
		ph.mu.Lock()
		if ph.reconnectTimer != nil {
			// Only counts if the reconnectTimer still exists. If not, a
			// connection _was_ somehow established.
			ph.reconnectTimer.Reset(ph.nextBackoff())
		}
		// Otherwise, someone else has stopped us so we can assume that
		// we're either connected or someone else will start us.
		ph.mu.Unlock()
	}

	// Always call this. We could have connected since we processed the
	// error.
	ph.stopIfConnected()
}

func (ph *peerHandler) stopIfConnected() {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if ph.reconnectTimer != nil && ph.host.Network().Connectedness(ph.peer) == network.Connected {
		logger.Debugw("successfully reconnected", "peer", ph.peer)
		ph.reconnectTimer.Stop()
		ph.reconnectTimer = nil
		ph.nextDelay = initialDelay
	}
}

// startIfDisconnected is the inverse of stopIfConnected.
func (ph *peerHandler) startIfDisconnected() {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if ph.reconnectTimer == nil && ph.host.Network().Connectedness(ph.peer) != network.Connected {
		logger.Debugw("disconnected from peer", "peer", ph.peer)
		// Always start with a short timeout so we can stagger things a bit.
		ph.reconnectTimer = time.AfterFunc(ph.nextBackoff(), ph.reconnect)
	}
}

// PeeringService maintains connections to specified peers, reconnecting on
// disconnect with a back-off.
type PeeringService struct {
	host host.Host

	mu    sync.RWMutex
	peers map[peer.ID]*peerHandler
	state state
}

// NewPeeringService constructs a new peering service. Peers can be added and
// removed immediately, but connections won't be formed until `Start` is called.
func NewPeeringService(host host.Host) *PeeringService {
	return &PeeringService{host: host, peers: make(map[peer.ID]*peerHandler)}
}

// Start starts the peering service, connecting and maintaining connections to
// all registered peers. It returns an error if the service has already been
// stopped.
func (ps *PeeringService) Start() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	switch ps.state {
	case stateInit:
		logger.Infow("starting")
	case stateRunning:
		return nil
	case stateStopped:
		return errors.New("already stopped")
	}
	ps.host.Network().Notify((*netNotifee)(ps))
	ps.state = stateRunning
	for _, handler := range ps.peers {
		go handler.startIfDisconnected()
	}
	return nil
}

// Stop stops the peering service.
func (ps *PeeringService) Stop() error {
	ps.host.Network().StopNotify((*netNotifee)(ps))

	ps.mu.Lock()
	defer ps.mu.Unlock()

	switch ps.state {
	case stateInit, stateRunning:
		logger.Infow("stopping")
		for _, handler := range ps.peers {
			handler.stop()
		}
		ps.state = stateStopped
	}
	return nil
}

// AddPeer adds a peer to the peering service. This function may be safely
// called at any time: before the service is started, while running, or after it
// stops.
//
// Add peer may also be called multiple times for the same peer. The new
// addresses will replace the old.
func (ps *PeeringService) AddPeer(info peer.AddrInfo) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if handler, ok := ps.peers[info.ID]; ok {
		logger.Infow("updating addresses", "peer", info.ID, "addrs", info.Addrs)
		handler.setAddrs(info.Addrs)
	} else {
		logger.Infow("peer added", "peer", info.ID, "addrs", info.Addrs)
		ps.host.ConnManager().Protect(info.ID, connmgrTag)

		handler = &peerHandler{
			host:      ps.host,
			peer:      info.ID,
			addrs:     info.Addrs,
			nextDelay: initialDelay,
		}
		handler.ctx, handler.cancel = context.WithCancel(context.Background())
		ps.peers[info.ID] = handler
		switch ps.state {
		case stateRunning:
			go handler.startIfDisconnected()
		case stateStopped:
			// We still construct everything in this state because
			// it's easier to reason about. But we should still free
			// resources.
			handler.cancel()
		}
	}
}

// RemovePeer removes a peer from the peering service. This function may be
// safely called at any time: before the service is started, while running, or
// after it stops.
func (ps *PeeringService) RemovePeer(id peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if handler, ok := ps.peers[id]; ok {
		logger.Infow("peer removed", "peer", id)
		ps.host.ConnManager().Unprotect(id, connmgrTag)

		handler.stop()
		delete(ps.peers, id)
	}
}

type netNotifee PeeringService

func (nn *netNotifee) Connected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	p := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if handler, ok := ps.peers[p]; ok {
		// use a goroutine to avoid blocking events.
		go handler.stopIfConnected()
	}
}
func (nn *netNotifee) Disconnected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	p := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if handler, ok := ps.peers[p]; ok {
		// use a goroutine to avoid blocking events.
		go handler.startIfDisconnected()
	}
}
func (nn *netNotifee) OpenedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) ClosedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) Listen(network.Network, multiaddr.Multiaddr)      {}
func (nn *netNotifee) ListenClose(network.Network, multiaddr.Multiaddr) {}
