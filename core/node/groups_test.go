package node

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleartextWSListeners(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		swarm []string
		want  []string
	}{
		{
			name:  "tcp listener gets ws sibling",
			swarm: []string{"/ip4/0.0.0.0/tcp/4001"},
			want:  []string{"/ip4/0.0.0.0/tcp/4001/ws"},
		},
		{
			name: "every tcp listener covered, non-tcp untouched",
			swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip6/::/tcp/4001",
				"/ip4/0.0.0.0/udp/4001/quic-v1",
			},
			want: []string{
				"/ip4/0.0.0.0/tcp/4001/ws",
				"/ip6/::/tcp/4001/ws",
			},
		},
		{
			name: "existing cleartext ws suppresses the append entirely",
			swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/tcp/8081/ws",
			},
			want: nil,
		},
		{
			name: "tls ws is not cleartext and does not suppress",
			swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/tcp/4001/tls/ws",
			},
			want: []string{"/ip4/0.0.0.0/tcp/4001/ws"},
		},
		{
			name: "wss is not cleartext and does not suppress",
			swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/dns4/example.com/tcp/443/wss",
			},
			want: []string{"/ip4/0.0.0.0/tcp/4001/ws"},
		},
		{
			name: "autowss wildcard does not suppress and is not duplicated",
			swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/tcp/4001/tls/sni/*.libp2p.direct/ws",
			},
			want: []string{"/ip4/0.0.0.0/tcp/4001/ws"},
		},
		{
			name:  "dns tcp listener gets ws sibling",
			swarm: []string{"/dns4/example.com/tcp/443"},
			want:  []string{"/dns4/example.com/tcp/443/ws"},
		},
		{
			name:  "no tcp listeners yields nothing",
			swarm: []string{"/ip4/0.0.0.0/udp/4001/quic-v1"},
			want:  nil,
		},
		{
			name:  "empty swarm yields nothing",
			swarm: nil,
			want:  nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, c.want, cleartextWSListeners(c.swarm))
		})
	}
}
