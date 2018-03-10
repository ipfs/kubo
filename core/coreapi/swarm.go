package coreapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	iaddr "gx/ipfs/QmQViVWBHbU6HmYjXcdNq7tVASCNgdg64ZGcauuDkLCivW/go-ipfs-addr"
	swarm "gx/ipfs/QmSwZMWwFZSUpe5muU2xgTUwppH24KfMwdPXiwbEp2c6G5/go-libp2p-swarm"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
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

	snet, ok := api.node.PeerHost.Network().(*swarm.Network)
	if !ok {
		return fmt.Errorf("peerhost network was not swarm")
	}

	swrm := snet.Swarm()

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

		swcon, ok := c.(*swarm.Conn)
		if ok {
			ci.muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
		}

		out = append(out, ci)
	}

	return out, nil
}

func (ci *connInfo) ID() peer.ID {
	return ci.ID()
}

func (ci *connInfo) Address() ma.Multiaddr {
	return ci.addr
}

func (ci *connInfo) Latency(context.Context) (time.Duration, error) {
	return ci.api.node.Peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams(context.Context) ([]string, error) {
	streams, err := ci.conn.GetStreams()
	if err != nil {
		return nil, err
	}

	out := make([]string, len(streams))
	for i, s := range streams {
		out[i] = string(s.Protocol())
	}

	return out, nil
}
