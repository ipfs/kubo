package coreapi

import (
	"context"
	"sort"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	inet "gx/ipfs/QmSTaEYUgDe1r581hxyd2u9582Hgp3KX4wGwYbRqz2u9Qh/go-libp2p-net"
	net "gx/ipfs/QmSTaEYUgDe1r581hxyd2u9582Hgp3KX4wGwYbRqz2u9Qh/go-libp2p-net"
	pstore "gx/ipfs/QmWtCpWB39Rzc2xTB75MKorsxNpo3TyecTEN24CJ3KVohE/go-libp2p-peerstore"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	swarm "gx/ipfs/QmYyFNDLTSy6rW99K8vHPvFPLa5bB4zoy8gGZENoiaBN6R/go-libp2p-swarm"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	iaddr "gx/ipfs/QmesXvbRGyKQn1XbPHx1Mr5E6RTJYR9c8zwFVuGZq9Aa1j/go-ipfs-addr"
)

type SwarmAPI CoreAPI

type connInfo struct {
	node *core.IpfsNode
	conn net.Conn
	dir  net.Direction

	addr  ma.Multiaddr
	peer  peer.ID
	muxer string
}

func (api *SwarmAPI) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	if api.node.PeerHost == nil {
		return coreiface.ErrOffline
	}

	if swrm, ok := api.node.PeerHost.Network().(*swarm.Swarm); ok {
		swrm.Backoff().Clear(pi.ID)
	}

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
	id := ia.ID()
	net := api.node.PeerHost.Network()

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
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	addrs := make(map[peer.ID][]ma.Multiaddr)
	ps := api.node.PeerHost.Network().Peerstore()
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
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.node.PeerHost.Addrs(), nil
}

func (api *SwarmAPI) ListenAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.node.PeerHost.Network().InterfaceListenAddresses()
}

func (api *SwarmAPI) Peers(context.Context) ([]coreiface.ConnectionInfo, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	conns := api.node.PeerHost.Network().Conns()

	var out []coreiface.ConnectionInfo
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := &connInfo{
			node: api.node,
			conn: c,
			dir:  c.Stat().Direction,

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
	return ci.node.Peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams() ([]protocol.ID, error) {
	streams := ci.conn.GetStreams()

	out := make([]protocol.ID, len(streams))
	for i, s := range streams {
		out[i] = s.Protocol()
	}

	return out, nil
}
