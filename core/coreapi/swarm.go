package coreapi

import (
	"context"
	"sort"
	"time"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	coreiface "gx/ipfs/QmNt8iUv3uJoVrJ3Ls4cgMLk124V4Bt1JxuUMWbjULt9ns/interface-go-ipfs-core"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	inet "gx/ipfs/QmTGxDz2CjBucFzPNTiWwzQmTWdrBnzqbqrMucDYMsjuPb/go-libp2p-net"
	net "gx/ipfs/QmTGxDz2CjBucFzPNTiWwzQmTWdrBnzqbqrMucDYMsjuPb/go-libp2p-net"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	swarm "gx/ipfs/QmU7iTrsNaJfu1Rf5DrvaJLH9wJtQwmP4Dj8oPduprAU68/go-libp2p-swarm"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	iaddr "gx/ipfs/QmdzbPrpnHjzaty3FHa5UYt66wesUMa5AuDDqkvigeGKYN/go-ipfs-addr"
)

type SwarmAPI CoreAPI

type connInfo struct {
	peerstore pstore.Peerstore
	conn      net.Conn
	dir       net.Direction

	addr  ma.Multiaddr
	peer  peer.ID
	muxer string
}

func (api *SwarmAPI) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	if api.peerHost == nil {
		return coreiface.ErrOffline
	}

	if swrm, ok := api.peerHost.Network().(*swarm.Swarm); ok {
		swrm.Backoff().Clear(pi.ID)
	}

	return api.peerHost.Connect(ctx, pi)
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr ma.Multiaddr) error {
	if api.peerHost == nil {
		return coreiface.ErrOffline
	}

	ia, err := iaddr.ParseMultiaddr(ma.Multiaddr(addr))
	if err != nil {
		return err
	}

	taddr := ia.Transport()
	id := ia.ID()
	net := api.peerHost.Network()

	if taddr == nil {
		if net.Connectedness(id) != inet.Connected {
			return coreiface.ErrNotConnected
		} else if err := net.ClosePeer(id); err != nil {
			return err
		}
	} else {
		for _, conn := range net.ConnsToPeer(id) {
			if !conn.RemoteMultiaddr().Equal(taddr) {
				continue
			}

			return conn.Close()
		}

		return coreiface.ErrConnNotFound
	}

	return nil
}

func (api *SwarmAPI) KnownAddrs(context.Context) (map[peer.ID][]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	addrs := make(map[peer.ID][]ma.Multiaddr)
	ps := api.peerHost.Network().Peerstore()
	for _, p := range ps.Peers() {
		for _, a := range ps.Addrs(p) {
			addrs[p] = append(addrs[p], a)
		}
		sort.Slice(addrs[p], func(i, j int) bool {
			return addrs[p][i].String() < addrs[p][j].String()
		})
	}

	return addrs, nil
}

func (api *SwarmAPI) LocalAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.peerHost.Addrs(), nil
}

func (api *SwarmAPI) ListenAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.peerHost.Network().InterfaceListenAddresses()
}

func (api *SwarmAPI) Peers(context.Context) ([]coreiface.ConnectionInfo, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	conns := api.peerHost.Network().Conns()

	var out []coreiface.ConnectionInfo
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := &connInfo{
			peerstore: api.peerstore,
			conn:      c,
			dir:       c.Stat().Direction,

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

func (ci *connInfo) Direction() net.Direction {
	return ci.dir
}

func (ci *connInfo) Latency() (time.Duration, error) {
	return ci.peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams() ([]protocol.ID, error) {
	streams := ci.conn.GetStreams()

	out := make([]protocol.ID, len(streams))
	for i, s := range streams {
		out[i] = s.Protocol()
	}

	return out, nil
}
