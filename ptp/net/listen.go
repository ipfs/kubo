package net

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/ptp"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

// NewListener creates new ptp listener
func NewListener(n *core.IpfsNode, proto string, addr ma.Multiaddr) (*ptp.ListenerInfo, error) {
	listener, err := Listen(n, proto)
	if err != nil {
		return nil, err
	}

	listenerInfo := ptp.ListenerInfo{
		Identity: n.Identity,
		Protocol: proto,
		Address:  addr,
		Closer:   listener,
		Running:  true,
		Registry: &n.PTP.Listeners,
	}

	go acceptStreams(n, &listenerInfo, listener)

	n.PTP.Listeners.Register(&listenerInfo)

	return &listenerInfo, nil
}

func acceptStreams(n *core.IpfsNode, listenerInfo *ptp.ListenerInfo, listener Listener) {
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

		stream := ptp.StreamInfo{
			Protocol: listenerInfo.Protocol,

			LocalPeer: listenerInfo.Identity,
			LocalAddr: listenerInfo.Address,

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &n.PTP.Streams,
		}

		n.PTP.Streams.Register(&stream)
		startStreaming(&stream)
	}
	n.PTP.Listeners.Deregister(listenerInfo.Protocol)
}
