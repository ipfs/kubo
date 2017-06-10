package ptp

import (
	"context"
	"errors"
	"time"

	net "gx/ipfs/QmRscs8KxrSmSv4iuevHv8JfuUzHBMoqiaHzxfDRiksd6e/go-libp2p-net"
	p2phost "gx/ipfs/QmUywuGNZoUKV8B9iyvup9bPkLiMrhTsyVMkeSXW5VxAfC/go-libp2p-host"
	pstore "gx/ipfs/QmXZSd1qR5BxZkPyuwfT5jpqQFScZccoZvDneXsKzCNHWX/go-libp2p-peerstore"
	pro "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

// PTP structure holds information on currently running streams/listeners
type PTP struct {
	Listeners ListenerRegistry
	Streams   StreamRegistry

	identity  peer.ID
	peerHost  p2phost.Host
	peerstore pstore.Peerstore
}

// NewPTP creates new PTP struct
func NewPTP(identity peer.ID, peerHost p2phost.Host, peerstore pstore.Peerstore) *PTP {
	return &PTP{
		identity:  identity,
		peerHost:  peerHost,
		peerstore: peerstore,
	}
}

func (ptp *PTP) newStreamTo(ctx2 context.Context, p peer.ID, protocol string) (net.Stream, error) {
	ctx, cancel := context.WithTimeout(ctx2, time.Second*30) //TODO: configurable?
	defer cancel()
	err := ptp.peerHost.Connect(ctx, pstore.PeerInfo{ID: p})
	if err != nil {
		return nil, err
	}
	return ptp.peerHost.NewStream(ctx2, p, pro.ID(protocol))
}

// Dial creates new P2P stream to a remote listener
func (ptp *PTP) Dial(ctx context.Context, addr ma.Multiaddr, peer peer.ID, proto string, bindAddr ma.Multiaddr) (*ListenerInfo, error) {
	lnet, _, err := manet.DialArgs(bindAddr)
	if err != nil {
		return nil, err
	}

	listenerInfo := ListenerInfo{
		Identity: ptp.identity,
		Protocol: proto,
	}

	remote, err := ptp.newStreamTo(ctx, peer, proto)
	if err != nil {
		return nil, err
	}

	switch lnet {
	case "tcp", "tcp4", "tcp6":
		listener, err := manet.Listen(bindAddr)
		if err != nil {
			if err2 := remote.Close(); err2 != nil {
				return nil, err2
			}
			return nil, err
		}

		listenerInfo.Address = listener.Multiaddr()
		listenerInfo.Closer = listener
		listenerInfo.Running = true

		go ptp.doAccept(&listenerInfo, remote, listener)

	default:
		return nil, errors.New("unsupported protocol: " + lnet)
	}

	return &listenerInfo, nil
}

func (ptp *PTP) doAccept(listenerInfo *ListenerInfo, remote net.Stream, listener manet.Listener) {
	defer listener.Close()

	local, err := listener.Accept()
	if err != nil {
		return
	}

	stream := StreamInfo{
		Protocol: listenerInfo.Protocol,

		LocalPeer: listenerInfo.Identity,
		LocalAddr: listenerInfo.Address,

		RemotePeer: remote.Conn().RemotePeer(),
		RemoteAddr: remote.Conn().RemoteMultiaddr(),

		Local:  local,
		Remote: remote,

		Registry: &ptp.Streams,
	}

	ptp.Streams.Register(&stream)
	stream.startStreaming()
}

// Listener wraps stream handler into a listener
type Listener interface {
	Accept() (net.Stream, error)
	Close() error
}

// P2PListener holds information on a listener
type P2PListener struct {
	peerHost p2phost.Host
	conCh    chan net.Stream
	proto    pro.ID
	ctx      context.Context
	cancel   func()
}

// Accept waits for a connection from the listener
func (il *P2PListener) Accept() (net.Stream, error) {
	select {
	case c := <-il.conCh:
		return c, nil
	case <-il.ctx.Done():
		return nil, il.ctx.Err()
	}
}

// Close closes the listener and removes stream handler
func (il *P2PListener) Close() error {
	il.cancel()
	il.peerHost.RemoveStreamHandler(il.proto)
	return nil
}

// Listen creates new P2PListener
func (ptp *PTP) registerStreamHandler(ctx2 context.Context, protocol string) (*P2PListener, error) {
	ctx, cancel := context.WithCancel(ctx2)

	list := &P2PListener{
		peerHost: ptp.peerHost,
		proto:    pro.ID(protocol),
		conCh:    make(chan net.Stream),
		ctx:      ctx,
		cancel:   cancel,
	}

	ptp.peerHost.SetStreamHandler(list.proto, func(s net.Stream) {
		select {
		case list.conCh <- s:
		case <-ctx.Done():
			s.Close()
		}
	})

	return list, nil
}

// NewListener creates new ptp listener
func (ptp *PTP) NewListener(ctx context.Context, proto string, addr ma.Multiaddr) (*ListenerInfo, error) {
	listener, err := ptp.registerStreamHandler(ctx, proto)
	if err != nil {
		return nil, err
	}

	listenerInfo := ListenerInfo{
		Identity: ptp.identity,
		Protocol: proto,
		Address:  addr,
		Closer:   listener,
		Running:  true,
		Registry: &ptp.Listeners,
	}

	go ptp.acceptStreams(&listenerInfo, listener)

	ptp.Listeners.Register(&listenerInfo)

	return &listenerInfo, nil
}

func (ptp *PTP) acceptStreams(listenerInfo *ListenerInfo, listener Listener) {
	for listenerInfo.Running {
		remote, err := listener.Accept()
		if err != nil {
			listener.Close()
			break
		}

		local, err := manet.Dial(listenerInfo.Address)
		if err != nil {
			remote.Close()
			continue
		}

		stream := StreamInfo{
			Protocol: listenerInfo.Protocol,

			LocalPeer: listenerInfo.Identity,
			LocalAddr: listenerInfo.Address,

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &ptp.Streams,
		}

		ptp.Streams.Register(&stream)
		stream.startStreaming()
	}
	ptp.Listeners.Deregister(listenerInfo.Protocol)
}

// CheckProtoExists checks whether a protocol handler is registered to
// mux handler
func (ptp *PTP) CheckProtoExists(proto string) bool {
	protos := ptp.peerHost.Mux().Protocols()

	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}
