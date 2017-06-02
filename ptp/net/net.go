package net

import (
	"time"

	context "context"
	core "github.com/ipfs/go-ipfs/core"

	net "gx/ipfs/QmRscs8KxrSmSv4iuevHv8JfuUzHBMoqiaHzxfDRiksd6e/go-libp2p-net"
	pstore "gx/ipfs/QmXZSd1qR5BxZkPyuwfT5jpqQFScZccoZvDneXsKzCNHWX/go-libp2p-peerstore"
	pro "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

// Listener wraps stream handler into a listener
type Listener interface {
	Accept() (net.Stream, error)
	Close() error
}

// IpfsListener holds information on a listener
type IpfsListener struct {
	node   *core.IpfsNode
	conCh  chan net.Stream
	proto  pro.ID
	ctx    context.Context
	cancel func()
}

// Accept waits for a connection from the listener
func (il *IpfsListener) Accept() (net.Stream, error) {
	select {
	case c := <-il.conCh:
		return c, nil
	case <-il.ctx.Done():
		return nil, il.ctx.Err()
	}
}

// Close closes the listener and removes stream handler
func (il *IpfsListener) Close() error {
	il.cancel()
	il.node.PeerHost.RemoveStreamHandler(il.proto)
	return nil
}

// Listen creates new IpfsListener
func Listen(nd *core.IpfsNode, protocol string) (*IpfsListener, error) {
	ctx, cancel := context.WithCancel(nd.Context())

	list := &IpfsListener{
		node:   nd,
		proto:  pro.ID(protocol),
		conCh:  make(chan net.Stream),
		ctx:    ctx,
		cancel: cancel,
	}

	nd.PeerHost.SetStreamHandler(list.proto, func(s net.Stream) {
		select {
		case list.conCh <- s:
		case <-ctx.Done():
			s.Close()
		}
	})

	return list, nil
}

// Dial dials to a specified node and protocol
func dial(nd *core.IpfsNode, p peer.ID, protocol string) (net.Stream, error) {
	ctx, cancel := context.WithTimeout(nd.Context(), time.Second*30)
	defer cancel()
	err := nd.PeerHost.Connect(ctx, pstore.PeerInfo{ID: p})
	if err != nil {
		return nil, err
	}
	return nd.PeerHost.NewStream(nd.Context(), p, pro.ID(protocol))
}

// CheckProtoExists checks whether a protocol handler is registered to
// mux handler
func CheckProtoExists(n *core.IpfsNode, proto string) bool {
	protos := n.PeerHost.Mux().Protocols()

	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}
