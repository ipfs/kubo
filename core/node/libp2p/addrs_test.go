package libp2p

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// mustMultiaddrs parses a list of multiaddr strings or fails the test.
func mustMultiaddrs(t *testing.T, addrs ...string) []ma.Multiaddr {
	t.Helper()
	out := make([]ma.Multiaddr, 0, len(addrs))
	for _, s := range addrs {
		m, err := ma.NewMultiaddr(s)
		require.NoError(t, err, "parse %q", s)
		out = append(out, m)
	}
	return out
}

func TestFindDeadListeners(t *testing.T) {
	cases := []struct {
		name        string
		listenAddrs []ma.Multiaddr
		addrFilters []string
		noAnnounce  []string
		want        []deadListenerFinding
	}{
		{
			name:        "empty config produces no findings",
			listenAddrs: mustMultiaddrs(t, "/ip4/192.168.1.5/tcp/4001"),
		},
		{
			name:        "loopback listener with loopback AddrFilters: one finding",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Source: deadListenerSourceAddrFilters, Rule: "/ip4/127.0.0.0/ipcidr/8"},
			},
		},
		{
			name:        "loopback NoAnnounce match alone is operator-intent: skipped",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			noAnnounce:  []string{"/ip4/127.0.0.0/ipcidr/8"},
		},
		{
			name:        "loopback in both lists: AddrFilters reported, NoAnnounce skipped",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			noAnnounce:  []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Source: deadListenerSourceAddrFilters, Rule: "/ip4/127.0.0.0/ipcidr/8"},
			},
		},
		{
			name:        "non-loopback NoAnnounce match is reported",
			listenAddrs: mustMultiaddrs(t, "/ip4/192.168.1.5/tcp/4001"),
			noAnnounce:  []string{"/ip4/192.168.0.0/ipcidr/16"},
			want: []deadListenerFinding{
				{Listener: "/ip4/192.168.1.5/tcp/4001", Source: deadListenerSourceNoAnnounce, Rule: "/ip4/192.168.0.0/ipcidr/16"},
			},
		},
		{
			name:        "IPv6 loopback (resolved from `::`) with `::/3` AddrFilters: flagged",
			listenAddrs: mustMultiaddrs(t, "/ip6/::1/tcp/4001"),
			addrFilters: []string{"/ip6/::/ipcidr/3"},
			want: []deadListenerFinding{
				{Listener: "/ip6/::1/tcp/4001", Source: deadListenerSourceAddrFilters, Rule: "/ip6/::/ipcidr/3"},
			},
		},
		{
			name:        "IPv6 loopback NoAnnounce-only is operator-intent: skipped",
			listenAddrs: mustMultiaddrs(t, "/ip6/::1/tcp/4001"),
			noAnnounce:  []string{"/ip6/::/ipcidr/3"},
		},
		{
			name:        "globally-routable IPv6 (resolved from `::`) is not flagged by `::/3`",
			listenAddrs: mustMultiaddrs(t, "/ip6/2604:2dc0:200:484::1/tcp/4001"),
			addrFilters: []string{"/ip6/::/ipcidr/3"},
		},
		{
			name:        "private LAN listener with matching private CIDR: flagged on AddrFilters",
			listenAddrs: mustMultiaddrs(t, "/ip4/192.168.1.5/tcp/4001"),
			addrFilters: []string{"/ip4/192.168.0.0/ipcidr/16"},
			want: []deadListenerFinding{
				{Listener: "/ip4/192.168.1.5/tcp/4001", Source: deadListenerSourceAddrFilters, Rule: "/ip4/192.168.0.0/ipcidr/16"},
			},
		},
		{
			name:        "DNS listener has no IP component: no finding",
			listenAddrs: mustMultiaddrs(t, "/dns/example.com/tcp/443/wss"),
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
		},
		{
			name:        "exact-match NoAnnounce entry is skipped (operator-explicit)",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			noAnnounce:  []string{"/ip4/127.0.0.1/tcp/8081/ws"},
		},
		{
			name:        "malformed filter entry: skipped, valid filters still match",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			addrFilters: []string{"garbage", "/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Source: deadListenerSourceAddrFilters, Rule: "/ip4/127.0.0.0/ipcidr/8"},
			},
		},
		{
			name: "bootstrapper-style mix: only AddrFilters loopback fires",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/147.135.44.132/tcp/4001",
				"/ip4/127.0.0.1/tcp/8081/ws",
				"/ip6/2604:2dc0:200:484::1/tcp/4001",
				"/ip6/::1/tcp/4001",
			),
			addrFilters: []string{
				"/ip4/127.0.0.0/ipcidr/8",
				"/ip6/::/ipcidr/3",
			},
			noAnnounce: []string{
				"/ip4/127.0.0.0/ipcidr/8",
				"/ip6/::/ipcidr/3",
			},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Source: deadListenerSourceAddrFilters, Rule: "/ip4/127.0.0.0/ipcidr/8"},
				{Listener: "/ip6/::1/tcp/4001", Source: deadListenerSourceAddrFilters, Rule: "/ip6/::/ipcidr/3"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findDeadListeners(tc.listenAddrs, tc.addrFilters, tc.noAnnounce)
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

// makeAddrsFactory must drop empty multiaddrs from the input list.
// A zero-component Multiaddr would otherwise reach the host's signed
// peer record and propagate to peers as "/" when they decode the wire
// bytes.
//
// See https://github.com/libp2p/js-libp2p/issues/3478#issuecomment-4322093929
func TestMakeAddrsFactoryDropsEmptyMultiaddrs(t *testing.T) {
	factory, err := makeAddrsFactory(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	good, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	if err != nil {
		t.Fatal(err)
	}

	in := []ma.Multiaddr{nil, good, {}, good}
	out := factory(in)

	if len(out) != 2 {
		t.Fatalf("expected 2 addrs after factory filter, got %d: %v", len(out), out)
	}
	for i, a := range out {
		if len(a) == 0 {
			t.Fatalf("factory returned an empty multiaddr at index %d", i)
		}
	}
}
