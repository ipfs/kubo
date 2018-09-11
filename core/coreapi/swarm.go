package coreapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	swarm "gx/ipfs/QmPQoCVRHaGD25VffyB7DFV5qP65hFSQJdSDy75P1vYBKe/go-libp2p-swarm"
	iaddr "gx/ipfs/QmSzdvo9aPzLj4HXWTcgGAp8N84tZc8LbLmFZFwUb1dpWk/go-ipfs-addr"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	pstore "gx/ipfs/QmfAQMFpgDU2U4BXG64qVr8HSiictfWvkSBz7Y2oDj65st/go-libp2p-peerstore"
	net "gx/ipfs/QmfDPh144WGBqRxZb1TGDHerbMnZATrHZggAPw7putNnBq/go-libp2p-net"
)

type SwarmAPI struct {
	*CoreAPI
}

type connInfo struct {
	api  *CoreAPI
	conn net.Conn

	addr  ma.Multiaddr
	peer  peer.ID
	muxer string
}

func (api *SwarmAPI) Connect(ctx context.Context, addr ma.Multiaddr) error {
	if api.node.PeerHost == nil {
		return coreiface.ErrOffline
	}

	swrm, ok := api.node.PeerHost.Network().(*swarm.Swarm)
	if !ok {
		return fmt.Errorf("peerhost network was not swarm")
	}

	ia, err := iaddr.ParseMultiaddr(ma.Multiaddr(addr))
	if err != nil {
		return err
	}

	pi := pstore.PeerInfo{
		ID:    ia.ID(),
		Addrs: []ma.Multiaddr{ia.Transport()},
	}

	swrm.Backoff().Clear(pi.ID)

	return api.node.PeerHost.Connect(ctx, pi)
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr ma.Multiaddr) error {
	if api.node.PeerHost == nil {
		return coreiface.ErrOffline
	}

	ia, err := iaddr.ParseMultiaddr(ma.Multiaddr(addr))
	if err != nil {
		return err
	}

	taddr := ia.Transport()

	found := false
	conns := api.node.PeerHost.Network().ConnsToPeer(ia.ID())
	for _, conn := range conns {
		if !conn.RemoteMultiaddr().Equal(taddr) {
			continue
		}

		if err := conn.Close(); err != nil {
			return err
		}
		found = true
		break
	}

	if !found {
		return errors.New("conn not found")
	}

	return nil
}

func (api *SwarmAPI) Peers(context.Context) ([]coreiface.PeerInfo, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	conns := api.node.PeerHost.Network().Conns()

	var out []coreiface.PeerInfo
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := &connInfo{
			api:  api.CoreAPI,
			conn: c,

			addr: addr,
			peer: pid,
		}

		/*
			// FIXME(steb):
			swcon, ok := c.(*swarm.Conn)
			if ok {
				ci.muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
			}
		*/

		out = append(out, ci)
	}

	return out, nil
}

func (ci *connInfo) ID() peer.ID {
	return ci.peer
}

func (ci *connInfo) Address() ma.Multiaddr {
	return ci.addr
}

func (ci *connInfo) Latency(context.Context) (time.Duration, error) {
	return ci.api.node.Peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams(context.Context) ([]string, error) {
	streams := ci.conn.GetStreams()

	out := make([]string, len(streams))
	for i, s := range streams {
		out[i] = string(s.Protocol())
	}

	return out, nil
}
