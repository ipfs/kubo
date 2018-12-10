package p2p

import (
	peer "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	pstore "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	p2phost "gx/ipfs/QmfD51tKgJiTMnW9JEiDiPwsCY4mqUoxkhKhBfyW12spTC/go-libp2p-host"
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
