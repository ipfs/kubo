package p2p

import (
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	p2phost "gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"
)

var log = logging.Logger("p2p-mount")

// P2P structure holds information on currently running streams/Listeners
type P2P struct {
	ListenersLocal *Listeners
	ListenersP2P   *Listeners
	Streams        *StreamRegistry

	identity  peer.ID
	peerHost  p2phost.Host
	peerstore pstore.Peerstore
}

// NewP2P creates new P2P struct
func NewP2P(identity peer.ID, peerHost p2phost.Host, peerstore pstore.Peerstore) *P2P {
	return &P2P{
		identity:  identity,
		peerHost:  peerHost,
		peerstore: peerstore,

		ListenersLocal: newListenersLocal(),
		ListenersP2P:   newListenersP2P(peerHost),

		Streams: &StreamRegistry{
			Streams:     map[uint64]*Stream{},
			ConnManager: peerHost.ConnManager(),
			conns:       map[peer.ID]int{},
		},
	}
}

// CheckProtoExists checks whether a proto handler is registered to
// mux handler
func (p2p *P2P) CheckProtoExists(proto string) bool {
	protos := p2p.peerHost.Mux().Protocols()

	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}
