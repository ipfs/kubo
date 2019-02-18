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

	return api.core().request("swarm/connect", saddrs...).Exec(ctx, nil)
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr multiaddr.Multiaddr) error {
	return api.core().request("swarm/disconnect", addr.String()).Exec(ctx, nil)
}

type streamInfo struct {
	Protocol string
}

type connInfo struct {
	Addr       string
	Peer       string
	JLatency   time.Duration `json:"Latency"`
	Muxer      string
	JDirection inet.Direction `json:"Direction"`
	JStreams   []streamInfo   `json:"Streams"`
}

func (c *connInfo) valid() error {
	_, err := multiaddr.NewMultiaddr(c.Addr)
	if err != nil {
		return err
	}

	_, err = peer.IDB58Decode(c.Peer)
	return err
}

func (c *connInfo) ID() peer.ID {
	id, _ := peer.IDB58Decode(c.Peer)
	return id
}

func (c *connInfo) Address() multiaddr.Multiaddr {
	a, _ := multiaddr.NewMultiaddr(c.Addr)
	return a
}

func (c *connInfo) Direction() inet.Direction {
	return c.JDirection
}

func (c *connInfo) Latency() (time.Duration, error) {
	return c.JLatency, nil
}

func (c *connInfo) Streams() ([]protocol.ID, error) {
	res := make([]protocol.ID, len(c.JStreams))
	for i, stream := range c.JStreams {
		res[i] = protocol.ID(stream.Protocol)
	}
	return res, nil
}

func (api *SwarmAPI) Peers(ctx context.Context) ([]iface.ConnectionInfo, error) {
	var out struct {
		Peers []*connInfo
	}

	err := api.core().request("swarm/peers").
		Option("streams", true).
		Option("latency", true).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	res := make([]iface.ConnectionInfo, len(out.Peers))
	for i, conn := range out.Peers {
		if err := conn.valid(); err != nil {
			return nil, err
		}
		res[i] = conn
	}

	return res, nil
}

func (api *SwarmAPI) KnownAddrs(ctx context.Context) (map[peer.ID][]multiaddr.Multiaddr, error) {
	var out struct {
		Addrs map[string][]string
	}
	if err := api.core().request("swarm/addrs").Exec(ctx, &out); err != nil {
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

	if err := api.core().request("swarm/addrs/local").Exec(ctx, &out); err != nil {
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

	if err := api.core().request("swarm/addrs/listen").Exec(ctx, &out); err != nil {
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
