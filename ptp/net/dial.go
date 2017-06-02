package net

import (
	"errors"

	core "github.com/ipfs/go-ipfs/core"
	ptp "github.com/ipfs/go-ipfs/ptp"

	net "gx/ipfs/QmRscs8KxrSmSv4iuevHv8JfuUzHBMoqiaHzxfDRiksd6e/go-libp2p-net"
	peerstore "gx/ipfs/QmXZSd1qR5BxZkPyuwfT5jpqQFScZccoZvDneXsKzCNHWX/go-libp2p-peerstore"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

func Dial(n *core.IpfsNode, addr ma.Multiaddr, peer peer.ID, proto string, bindAddr ma.Multiaddr) (*ptp.ListenerInfo, error) {
	lnet, _, err := manet.DialArgs(bindAddr)
	if err != nil {
		return nil, err
	}

	app := ptp.ListenerInfo{
		Identity: n.Identity,
		Protocol: proto,
	}

	n.Peerstore.AddAddr(peer, addr, peerstore.TempAddrTTL)

	remote, err := dial(n, peer, proto)
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

		app.Address = listener.Multiaddr()
		app.Closer = listener
		app.Running = true

		go doAccept(n, &app, remote, listener)

	default:
		return nil, errors.New("unsupported protocol: " + lnet)
	}

	return &app, nil
}

func doAccept(n *core.IpfsNode, app *ptp.ListenerInfo, remote net.Stream, listener manet.Listener) {
	defer listener.Close()

	local, err := listener.Accept()
	if err != nil {
		return
	}

	stream := ptp.StreamInfo{
		Protocol: app.Protocol,

		LocalPeer: app.Identity,
		LocalAddr: app.Address,

		RemotePeer: remote.Conn().RemotePeer(),
		RemoteAddr: remote.Conn().RemoteMultiaddr(),

		Local:  local,
		Remote: remote,

		Registry: &n.PTP.Streams,
	}

	n.PTP.Streams.Register(&stream)
	startStreaming(&stream)
}
