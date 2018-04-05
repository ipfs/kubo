package coreapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	net "gx/ipfs/QmZNJyx9GGCX4GeuHnLB8fxaxMLs4MjTjHokxfQcCd6Nve/go-libp2p-net"
	pstore "gx/ipfs/Qmda4cPRvSRyox3SqgJN6DfSZGU5TtHufPTp9uXjFj71X6/go-libp2p-peerstore"
	swarm "gx/ipfs/QmeDpqUwwdye8ABKVMPXKuWwPVURFdqTqssbTUB39E2Nwd/go-libp2p-swarm"
	iaddr "gx/ipfs/QmePSRaGafvmURQwQkHPDBJsaGwKXC1WpBBHVCQxdr8FPn/go-ipfs-addr"
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
	return ci.peer
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
