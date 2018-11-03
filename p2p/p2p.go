package p2p

import (
	pstore "gx/ipfs/QmNVzTd29YipdnuXB67b2VnFSerudTVS3ZrxyBhNxcdvAg/go-libp2p-peerstore"
	p2phost "gx/ipfs/QmUR3nQ8nSP7vUnzTvz2kLxq4ncud6eC2s4BAEC8uiAsde/go-libp2p-host"
	peer "gx/ipfs/QmUz7HmU4ws577TMEVNoTjNghVvq2jXnxnVhbkNXKPKpgK/go-libp2p-peer"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
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
