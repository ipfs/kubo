package net

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/corenet"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

// NewListener creates new corenet listener
func NewListener(n *core.IpfsNode, proto string, addr ma.Multiaddr) (*corenet.AppInfo, error) {
	listener, err := Listen(n, proto)
	if err != nil {
		return nil, err
	}

	app := corenet.AppInfo{
		Identity: n.Identity,
		Protocol: proto,
		Address:  addr,
		Closer:   listener,
		Running:  true,
		Registry: &n.Corenet.Apps,
	}

	go acceptStreams(n, &app, listener)

	n.Corenet.Apps.Register(&app)

	return &app, nil
}

func acceptStreams(n *core.IpfsNode, app *corenet.AppInfo, listener Listener) {
	for app.Running {
		remote, err := listener.Accept()
		if err != nil {
			listener.Close()
			break
		}

		local, err := manet.Dial(app.Address)
		if err != nil {
			remote.Close()
			continue
		}

		stream := corenet.StreamInfo{
			Protocol: app.Protocol,

			LocalPeer: app.Identity,
			LocalAddr: app.Address,

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &n.Corenet.Streams,
		}

		n.Corenet.Streams.Register(&stream)
		startStreaming(&stream)
	}
	n.Corenet.Apps.Deregister(app.Protocol)
}
