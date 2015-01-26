package basichost

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	goprocess "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"

	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	inat "github.com/jbenet/go-ipfs/p2p/nat"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	identify "github.com/jbenet/go-ipfs/p2p/protocol/identify"
	relay "github.com/jbenet/go-ipfs/p2p/protocol/relay"
)

var log = eventlog.Logger("p2p/host/basic")

// Option is a type used to pass in options to the host.
type Option int

const (
	// NATPortMap makes the host attempt to open port-mapping in NAT devices
	// for all its listeners. Pass in this option in the constructor to
	// asynchronously a) find a gateway, b) open port mappings, c) republish
	// port mappings periodically. The NATed addresses are included in the
	// Host's Addrs() list.
	NATPortMap Option = iota
)

// BasicHost is the basic implementation of the host.Host interface. This
// particular host implementation:
//  * uses a protocol muxer to mux per-protocol streams
//  * uses an identity service to send + receive node information
//  * uses a relay service to allow hosts to relay conns for each other
//  * uses a nat service to establish NAT port mappings
type BasicHost struct {
	network inet.Network
	mux     *protocol.Mux
	ids     *identify.IDService
	relay   *relay.RelayService

	natmu sync.Mutex
	nat   *inat.NAT

	proc goprocess.Process
}

// New constructs and sets up a new *BasicHost with given Network
func New(net inet.Network, opts ...Option) *BasicHost {
	h := &BasicHost{
		network: net,
		mux:     protocol.NewMux(),
	}

	h.proc = goprocess.WithTeardown(func() error {
		return h.Network().Close()
	})

	// setup host services
	h.ids = identify.NewIDService(h)
	h.relay = relay.NewRelayService(h, h.Mux().HandleSync)

	net.SetConnHandler(h.newConnHandler)
	net.SetStreamHandler(h.newStreamHandler)

	for _, o := range opts {
		switch o {
		case NATPortMap:
			h.setupNATPortMap()
		}
	}

	return h
}

func (h *BasicHost) setupNATPortMap() {
	// do this asynchronously to avoid blocking daemon startup

	h.proc.Go(func(worker goprocess.Process) {
		nat := inat.DiscoverNAT()
		if nat == nil { // no nat, or failed to get it.
			return
		}

		select {
		case <-worker.Closing():
			nat.Close()
			return
		default:
		}

		// wire up the nat to close when proc closes.
		h.proc.AddChild(nat.Process())

		h.natmu.Lock()
		h.nat = nat
		h.natmu.Unlock()

		addrs := h.Network().ListenAddresses()
		nat.PortMapAddrs(addrs)
		mapAddrs := nat.ExternalAddrs()
		if len(mapAddrs) > 0 {
			log.Infof("NAT mapping addrs: %s", mapAddrs)
		}
	})
}

// newConnHandler is the remote-opened conn handler for inet.Network
func (h *BasicHost) newConnHandler(c inet.Conn) {
	h.ids.IdentifyConn(c)
}

// newStreamHandler is the remote-opened stream handler for inet.Network
func (h *BasicHost) newStreamHandler(s inet.Stream) {
	h.Mux().Handle(s)
}

// ID returns the (local) peer.ID associated with this Host
func (h *BasicHost) ID() peer.ID {
	return h.Network().LocalPeer()
}

// Peerstore returns the Host's repository of Peer Addresses and Keys.
func (h *BasicHost) Peerstore() peer.Peerstore {
	return h.Network().Peerstore()
}

// Networks returns the Network interface of the Host
func (h *BasicHost) Network() inet.Network {
	return h.network
}

// Mux returns the Mux multiplexing incoming streams to protocol handlers
func (h *BasicHost) Mux() *protocol.Mux {
	return h.mux
}

func (h *BasicHost) IDService() *identify.IDService {
	return h.ids
}

// SetStreamHandler sets the protocol handler on the Host's Mux.
// This is equivalent to:
//   host.Mux().SetHandler(proto, handler)
// (Threadsafe)
func (h *BasicHost) SetStreamHandler(pid protocol.ID, handler inet.StreamHandler) {
	h.Mux().SetHandler(pid, handler)
}

// NewStream opens a new stream to given peer p, and writes a p2p/protocol
// header with given protocol.ID. If there is no connection to p, attempts
// to create one. If ProtocolID is "", writes no header.
// (Threadsafe)
func (h *BasicHost) NewStream(pid protocol.ID, p peer.ID) (inet.Stream, error) {
	s, err := h.Network().NewStream(p)
	if err != nil {
		return nil, err
	}

	if err := protocol.WriteHeader(s, pid); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// Connect ensures there is a connection between this host and the peer with
// given peer.ID. Connect will absorb the addresses in pi into its internal
// peerstore. If there is not an active connection, Connect will issue a
// h.Network.Dial, and block until a connection is open, or an error is
// returned. // TODO: Relay + NAT.
func (h *BasicHost) Connect(ctx context.Context, pi peer.PeerInfo) error {

	// absorb addresses into peerstore
	h.Peerstore().AddPeerInfo(pi)

	cs := h.Network().ConnsToPeer(pi.ID)
	if len(cs) > 0 {
		return nil
	}

	return h.dialPeer(ctx, pi.ID)
}

// dialPeer opens a connection to peer, and makes sure to identify
// the connection once it has been opened.
func (h *BasicHost) dialPeer(ctx context.Context, p peer.ID) error {
	log.Debugf("host %s dialing %s", h.ID, p)
	c, err := h.Network().DialPeer(ctx, p)
	if err != nil {
		return err
	}

	// identify the connection before returning.
	done := make(chan struct{})
	go func() {
		h.ids.IdentifyConn(c)
		close(done)
	}()

	// respect don contexteone
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Debugf("host %s finished dialing %s", h.ID, p)
	return nil
}

func (h *BasicHost) Addrs() []ma.Multiaddr {
	addrs, err := h.Network().InterfaceListenAddresses()
	if err != nil {
		log.Debug("error retrieving network interface addrs")
	}

	h.natmu.Lock()
	nat := h.nat
	h.natmu.Unlock()
	if nat != nil {
		addrs = append(addrs, nat.ExternalAddrs()...)
	}

	return addrs
}

// Close shuts down the Host's services (network, etc).
func (h *BasicHost) Close() error {
	return h.proc.Close()
}
