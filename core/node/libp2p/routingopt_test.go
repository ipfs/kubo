package libp2p

import (
	"testing"

	config "github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/require"
)

func TestHttpAddrsFromConfig(t *testing.T) {
	require.Equal(t, []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		httpAddrsFromConfig(config.Addresses{
			Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		}), "Swarm addrs should be taken by default")

	require.Equal(t, []string{"/ip4/192.168.0.1/tcp/4001"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:    []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			Announce: []string{"/ip4/192.168.0.1/tcp/4001"},
		}), "Announce addrs should override Swarm if specified")

	require.Equal(t, []string{"/ip4/0.0.0.0/udp/4001/quic-v1"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:      []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			NoAnnounce: []string{"/ip4/0.0.0.0/tcp/4001"},
		}), "Swarm addrs should not contain NoAnnounce addrs")

	require.Equal(t, []string{"/ip4/192.168.0.1/tcp/4001", "/ip4/192.168.0.2/tcp/4001"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:          []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			Announce:       []string{"/ip4/192.168.0.1/tcp/4001"},
			AppendAnnounce: []string{"/ip4/192.168.0.2/tcp/4001"},
		}), "AppendAnnounce addrs should be included if specified")
}
