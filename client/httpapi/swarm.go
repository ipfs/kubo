package httpapi

import (
	"context"
	"time"

	"github.com/ipfs/interface-go-ipfs-core"
	inet "github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/multiformats/go-multiaddr"
)

type SwarmAPI HttpApi

func (api *SwarmAPI) Connect(ctx context.Context, pi peerstore.PeerInfo) error {
	pidma, err := multiaddr.NewComponent("p2p", pi.ID.Pretty())
	if err != nil {
		return err
	}

	saddrs := make([]string, len(pi.Addrs))
	for i, addr := range pi.Addrs {
		saddrs[i] = addr.Encapsulate(pidma).String()
	}

	return api.core().Request("swarm/connect", saddrs...).Exec(ctx, nil)
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr multiaddr.Multiaddr) error {
	return api.core().Request("swarm/disconnect", addr.String()).Exec(ctx, nil)
}

type connInfo struct {
	addr      multiaddr.Multiaddr
	peer      peer.ID
	latency   time.Duration
	muxer     string
	direction inet.Direction
	streams   []protocol.ID
}

func (c *connInfo) ID() peer.ID {
	return c.peer
}

func (c *connInfo) Address() multiaddr.Multiaddr {
	return c.addr
}

func (c *connInfo) Direction() inet.Direction {
	return c.direction
}

func (c *connInfo) Latency() (time.Duration, error) {
	return c.latency, nil
}

func (c *connInfo) Streams() ([]protocol.ID, error) {
	return c.streams, nil
}

func (api *SwarmAPI) Peers(ctx context.Context) ([]iface.ConnectionInfo, error) {
	var resp struct {
		Peers []struct {
			Addr      string
			Peer      string
			Latency   time.Duration
			Muxer     string
			Direction inet.Direction
			Streams   []struct {
				Protocol string
			}
		}
	}

	err := api.core().Request("swarm/peers").
		Option("streams", true).
		Option("latency", true).
		Exec(ctx, &resp)
	if err != nil {
		return nil, err
	}

	res := make([]iface.ConnectionInfo, len(resp.Peers))
	for i, conn := range resp.Peers {
		out := &connInfo{
			latency:   conn.Latency,
			muxer:     conn.Muxer,
			direction: conn.Direction,
		}

		out.peer, err = peer.IDB58Decode(conn.Peer)
		if err != nil {
			return nil, err
		}

		out.addr, err = multiaddr.NewMultiaddr(conn.Addr)
		if err != nil {
			return nil, err
		}

		out.streams = make([]protocol.ID, len(conn.Streams))
		for i, p := range conn.Streams {
			out.streams[i] = protocol.ID(p.Protocol)
		}

		res[i] = out
	}

	return res, nil
}

func (api *SwarmAPI) KnownAddrs(ctx context.Context) (map[peer.ID][]multiaddr.Multiaddr, error) {
	var out struct {
		Addrs map[string][]string
	}
	if err := api.core().Request("swarm/addrs").Exec(ctx, &out); err != nil {
		return nil, err
	}
	res := map[peer.ID][]multiaddr.Multiaddr{}
	for spid, saddrs := range out.Addrs {
		addrs := make([]multiaddr.Multiaddr, len(saddrs))

		for i, addr := range saddrs {
			a, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				return nil, err
			}
			addrs[i] = a
		}

		pid, err := peer.IDB58Decode(spid)
		if err != nil {
			return nil, err
		}

		res[pid] = addrs
	}

	return res, nil
}

func (api *SwarmAPI) LocalAddrs(ctx context.Context) ([]multiaddr.Multiaddr, error) {
	var out struct {
		Strings []string
	}

	if err := api.core().Request("swarm/addrs/local").Exec(ctx, &out); err != nil {
		return nil, err
	}

	res := make([]multiaddr.Multiaddr, len(out.Strings))
	for i, addr := range out.Strings {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		res[i] = ma
	}
	return res, nil
}

func (api *SwarmAPI) ListenAddrs(ctx context.Context) ([]multiaddr.Multiaddr, error) {
	var out struct {
		Strings []string
	}

	if err := api.core().Request("swarm/addrs/listen").Exec(ctx, &out); err != nil {
		return nil, err
	}

	res := make([]multiaddr.Multiaddr, len(out.Strings))
	for i, addr := range out.Strings {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		res[i] = ma
	}
	return res, nil
}

func (api *SwarmAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
