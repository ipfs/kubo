package swarm

import (
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

// MaxConcurrentRequestsPerPeer defines the pipelining that we can do per-peer.
// the networking layer makes sure to provide proper backpressure to the remote
// side by only handling a max number of concurrent requests to completion.
const MaxConcurrentRequestsPerPeer = 20

// swarmPeer represents a connection to the outside world.
// Implements router.Node
type swarmPeer struct {
	swarm *Swarm
	conn  *conn.MultiConn
	cg    ctxgroup.ContextGroup
}

// newSwarmPeer constructs a new swarmPeer, and starts is worker.
// this doesn't connect it, or add it to the swarm's routing table.
// Implements router.Node
func newSwarmPeer(s *Swarm, p peer.Peer) (*swarmPeer, error) {
	c, err := conn.NewMultiConn(s.cg.Context(), s.LocalPeer(), p, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating MultiConn: %s", err)
	}

	sp := &swarmPeer{
		swarm: s,
		conn:  c,
		cg:    ctxgroup.WithParent(s.cg), // swarmPeer closes when swarm closes.
	}

	// kicks off the worker.
	// ctggroup makes sure swarmPeer doesn't close until this func returns
	sp.cg.AddChildFunc(sp.listen)
	return sp, nil
}

// LocalPeer returns the local peer
func (sp *swarmPeer) LocalPeer() peer.Peer {
	return sp.conn.LocalPeer()
}

// RemotePeer returns the peer we're connected to.
func (sp *swarmPeer) RemotePeer() peer.Peer {
	return sp.conn.RemotePeer()
}

// Address is the peer's ID
func (sp *swarmPeer) Address() router.Address {
	return sp.RemotePeer()
}

// Close closes the swarmPeer service
func (sp *swarmPeer) Close() error {
	return sp.cg.Close()
}

// list to the multiconn and route packets in.
func (sp *swarmPeer) listen(parent ctxgroup.ContextGroup) {

	// we listen and pipeline using:
	// - 1x listener (this function, the for loop below)
	// - 1x pipelining semaphore
	// - up to Nx goroutine pipeline workers
	// this approach is chosen over N persistent goroutines because
	// spawning a goroutine every time is cheaper than keeping N
	// additional goroutines all the time, for inactive connections

	pipelineSema := make(chan struct{}, MaxConcurrentRequestsPerPeer)
	for i := 0; i < MaxConcurrentRequestsPerPeer; i++ {
		pipelineSema <- struct{}{}
	}

	// the sad part of using io is we still need to consume msgs
	// using a context-less api, which means we need to use an
	// extra goroutine, to make sure we close the connection and
	// unlock our blocked listener.
	// (the Context just does not mix well with io.ReadWriters)
	// - conn.SetDeadline could be explored
	go func() {
		<-parent.Closing()
		sp.conn.Close()
	}()

	listener := func() {
		for {
			msg, err := sp.conn.ReadMsg()
			// we want this to happen before checking the error, as we may
			// be closing (which is not an error). any last message is dropped.
			select {
			case <-parent.Closing():
				return
			case <-sp.conn.Closing():
				return
			default:
			}

			if err != nil {
				log.Errorf("error receiving message from multiconn: %s", err)
				continue
			}

			select {
			case <-parent.Closing():
				return
			case <-pipelineSema: // acquire pipelining resource
				go func(m []byte) {
					sp.handleIncomingMessage(parent.Context(), m)
					pipelineSema <- struct{}{}
				}(msg)
			}
		}
	}

	// function call so that we can isolate the functionality,
	// and so we can call return in loops above and not confuse flow.
	listener()
}

func (sp *swarmPeer) handleIncomingMessage(ctx context.Context, msg []byte) {
	// handle incoming message message.
	// we derive a new context for this incoming request.
	ctx, _ = context.WithCancel(ctx)

	p := netmsg.Packet{
		Src:     sp.RemotePeer(),
		Dst:     sp.swarm.client().Address(),
		Data:    msg,
		Context: ctx,
	}

	// We also can't yet pass unread io.RW to the clients directly.
	// muxado, SPDY, QUIC, and other stream multiplexors could
	// make this a breeze.

	// this runs the entire request. it should not return until ALL action
	// is done. this is so that we rate limit and respond to backpressure well.
	// TODO: pipelining (handle up to N concurrent requests).
	// doing pipelining with SPDY or muxado is probably TRTTD.
	if err := sp.HandlePacket(&p, nil); err != nil {
		log.Errorf("error handling incoming request: %v", err)
	}

	// should be done with the underlying bytes. release (the kraken)!
	// TODO: enable this. there is a bug relating to mpool or something. swarm_tests fail.
	// sp.conn.ReleaseMsg(msg)
}

func (sp *swarmPeer) HandlePacket(p router.Packet, n router.Node) error {
	switch p.Destination() {
	case sp.swarm.client().Address(): // incoming
		return sp.swarm.HandlePacket(p, sp)

	case sp.RemotePeer(): // outgoing
		buf, ok := p.Payload().([]byte)
		if !ok {
			return netmsg.ErrInvalidPayload
		}
		if err := sp.conn.WriteMsg(buf); err != nil {
			return fmt.Errorf("swarmPeer error sending: %s", err)
		}
		return nil

	default: // problem
		return fmt.Errorf("swarmPeer routing error: %v got %v", sp, p)
	}
}
