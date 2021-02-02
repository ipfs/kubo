package libp2p

import (
	"context"
	"fmt"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

type HostOption func(ctx context.Context, id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error)

var DefaultHostOption HostOption = constructPeerHost

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
	golog.SetLogLevel("p2p/holepunch", "INFO")

	relayId, err := peer.Decode("Qma71QQyJN7Sw7gz1cgJ4C66ubHmvKqBasSegKRugM5qo6")
	if err != nil {
		panic(err)
	}
	relayInfo := []peer.AddrInfo{
		{
			ID:    relayId,
			Addrs: []ma.Multiaddr{ma.StringCast("/ip4/54.255.209.104/tcp/12001"), ma.StringCast("/ip4/54.255.209.104/udp/12001/quic")},
		},
	}

	pkey := ps.PrivKey(id)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
	}
	options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.EnableHolePunching(), libp2p.EnableAutoRelay(),
		libp2p.ForceReachabilityPrivate(), libp2p.StaticRelays(relayInfo),
		libp2p.Peerstore(ps)}, options...)
	return libp2p.New(ctx, options...)
}
