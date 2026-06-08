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
		swarmListen []string
		addrFilters []string
		noAnnounce  []string
		want        []deadListenerFinding
	}{
		{
			name:        "empty config produces no findings",
			listenAddrs: mustMultiaddrs(t, "/ip4/192.168.1.5/tcp/4001"),
			swarmListen: []string{"/ip4/192.168.1.5/tcp/4001"},
		},
		{
			name:        "explicit loopback listen with loopback AddrFilters: explicit AddrFilters finding (reverse-proxy gotcha)",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/ws"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			name: "wildcard listen resolves to loopback: non-explicit AddrFilters finding",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/127.0.0.1/tcp/4001",
				"/ip4/1.2.3.4/tcp/4001",
			),
			swarmListen: []string{"/ip4/0.0.0.0/tcp/4001"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/4001", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: false},
			},
		},
		{
			name:        "explicit loopback listen with loopback NoAnnounce: non-explicit NoAnnounce finding (debug trace)",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/ws"},
			noAnnounce:  []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceNoAnnounce, Explicit: true},
			},
		},
		{
			name: "wildcard listen resolves to loopback with NoAnnounce: non-explicit NoAnnounce finding",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/127.0.0.1/tcp/4001",
				"/ip4/1.2.3.4/tcp/4001",
			),
			swarmListen: []string{"/ip4/0.0.0.0/tcp/4001"},
			noAnnounce:  []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/4001", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceNoAnnounce, Explicit: false},
			},
		},
		{
			name:        "loopback in both AddrFilters and NoAnnounce on explicit listen: one finding per source",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/ws"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			noAnnounce:  []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceNoAnnounce, Explicit: true},
			},
		},
		{
			name: "wildcard IPv6 listen resolves to ULA with `fc00::/7` AddrFilters: non-explicit AddrFilters finding",
			listenAddrs: mustMultiaddrs(t,
				"/ip6/fd7d:54ce:fe4::1/tcp/4001",
				"/ip6/2604:2dc0:200:484::1/tcp/4001",
			),
			swarmListen: []string{"/ip6/::/tcp/4001"},
			addrFilters: []string{"/ip6/fc00::/ipcidr/7"},
			want: []deadListenerFinding{
				{Listener: "/ip6/fd7d:54ce:fe4::1/tcp/4001", Rule: "/ip6/fc00::/ipcidr/7", Source: deadListenerSourceAddrFilters, Explicit: false},
			},
		},
		{
			name:        "explicit Docker bridge listen with matching private CIDR: explicit AddrFilters finding",
			listenAddrs: mustMultiaddrs(t, "/ip4/172.17.0.1/tcp/4001"),
			swarmListen: []string{"/ip4/172.17.0.1/tcp/4001"},
			addrFilters: []string{"/ip4/172.16.0.0/ipcidr/12"},
			want: []deadListenerFinding{
				{Listener: "/ip4/172.17.0.1/tcp/4001", Rule: "/ip4/172.16.0.0/ipcidr/12", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			name:        "globally-routable IPv6 explicit listen is not matched by `::/3`",
			listenAddrs: mustMultiaddrs(t, "/ip6/2604:2dc0:200:484::1/tcp/4001"),
			swarmListen: []string{"/ip6/2604:2dc0:200:484::1/tcp/4001"},
			addrFilters: []string{"/ip6/::/ipcidr/3"},
		},
		{
			name:        "DNS listener has no IP component: no finding",
			listenAddrs: mustMultiaddrs(t, "/dns/example.com/tcp/443/wss"),
			swarmListen: []string{"/dns/example.com/tcp/443/wss"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
		},
		{
			name:        "exact-match NoAnnounce multiaddr is not a CIDR: skipped",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/ws"},
			noAnnounce:  []string{"/ip4/127.0.0.1/tcp/8081/ws"},
		},
		{
			name:        "malformed AddrFilters entry: skipped, valid filters still match",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/ws"},
			addrFilters: []string{"garbage", "/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			name: "server-profile bootstrapper mix: explicit reverse-proxy listen flagged ERROR, wildcard-resolved interfaces DEBUG",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/147.135.44.132/tcp/4001",
				"/ip4/127.0.0.1/tcp/4001", // loopback expansion of /ip4/0.0.0.0
				"/ip4/127.0.0.1/tcp/8081/ws",
				"/ip6/2604:2dc0:200:484::1/tcp/4001",
				"/ip6/::1/tcp/4001",
			),
			swarmListen: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/127.0.0.1/tcp/8081/ws",
				"/ip6/::/tcp/4001",
			},
			addrFilters: []string{
				"/ip4/127.0.0.0/ipcidr/8",
				"/ip6/::/ipcidr/3",
			},
			noAnnounce: []string{
				"/ip4/127.0.0.0/ipcidr/8",
				"/ip6/::/ipcidr/3",
			},
			// The /ip4/127.0.0.1/tcp/4001 loopback shares its IP with the
			// explicit /ws listener but came from the /ip4/0.0.0.0 wildcard,
			// so it stays non-explicit (DEBUG); only the /ws listener is ERROR.
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
				{Listener: "/ip4/127.0.0.1/tcp/4001", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: false},
				{Listener: "/ip6/::1/tcp/4001", Rule: "/ip6/::/ipcidr/3", Source: deadListenerSourceAddrFilters, Explicit: false},
				{Listener: "/ip4/127.0.0.1/tcp/8081/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceNoAnnounce, Explicit: true},
				{Listener: "/ip4/127.0.0.1/tcp/4001", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceNoAnnounce, Explicit: false},
				{Listener: "/ip6/::1/tcp/4001", Rule: "/ip6/::/ipcidr/3", Source: deadListenerSourceNoAnnounce, Explicit: false},
			},
		},
		// A listener is reported under a different multiaddr than its
		// Addresses.Swarm entry once a transport rewrites trailing
		// components. Matching on IP+port keeps the explicit listener
		// recognizable across these rewrites.
		{
			// A WebTransport listener reports the current and next cert
			// hashes, so InterfaceListenAddresses surfaces two /certhash
			// components the config entry never had.
			name:        "explicit WebTransport listen reported with /certhash: explicit AddrFilters finding",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/udp/4001/quic-v1/webtransport/certhash/uEiAkH5a4DPGKUuOBjYw0CgwjLa2R_RF71v86aVxlqdKNOQ/certhash/uEiAsGPzpiPGQzSlVHRXrUCT5EkTV7YFrV4VZ3hpEKTd_zg"),
			swarmListen: []string{"/ip4/127.0.0.1/udp/4001/quic-v1/webtransport"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/udp/4001/quic-v1/webtransport/certhash/uEiAkH5a4DPGKUuOBjYw0CgwjLa2R_RF71v86aVxlqdKNOQ/certhash/uEiAsGPzpiPGQzSlVHRXrUCT5EkTV7YFrV4VZ3hpEKTd_zg", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			// TCP and QUIC share port 4001 in stock Kubo. A pinned QUIC
			// listener must not promote the TCP wildcard's expansion onto
			// the same IP to ERROR: they are different sockets.
			name: "pinned QUIC listener leaves same-port TCP wildcard expansion non-explicit",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/172.17.0.1/tcp/4001",         // from the /ip4/0.0.0.0/tcp/4001 wildcard
				"/ip4/172.17.0.1/udp/4001/quic-v1", // the explicit QUIC listener
			),
			swarmListen: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/172.17.0.1/udp/4001/quic-v1",
			},
			addrFilters: []string{"/ip4/172.16.0.0/ipcidr/12"},
			want: []deadListenerFinding{
				{Listener: "/ip4/172.17.0.1/tcp/4001", Rule: "/ip4/172.16.0.0/ipcidr/12", Source: deadListenerSourceAddrFilters, Explicit: false},
				{Listener: "/ip4/172.17.0.1/udp/4001/quic-v1", Rule: "/ip4/172.16.0.0/ipcidr/12", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			name:        "explicit wss listen reported as /tls/ws: explicit AddrFilters finding",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/8081/tls/ws"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/8081/wss"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/8081/tls/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			// The wildcard expansion onto loopback (/tcp/4001) and the
			// explicit reverse-proxy wss listener (/tcp/8081) share an IP
			// but differ in port, so only the explicit port is ERROR.
			name: "wildcard expansion shares loopback IP with explicit wss listener on another port",
			listenAddrs: mustMultiaddrs(t,
				"/ip4/127.0.0.1/tcp/4001",        // from the /ip4/0.0.0.0 wildcard
				"/ip4/127.0.0.1/tcp/8081/tls/ws", // the explicit wss listener, as reported
			),
			swarmListen: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/127.0.0.1/tcp/8081/wss",
			},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/4001", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: false},
				{Listener: "/ip4/127.0.0.1/tcp/8081/tls/ws", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			// Uppercase IPv6 in config plus a wss->/tls/ws rewrite at the
			// listener: matching needs both IP canonicalization and the
			// transport-independent endpoint key.
			name:        "explicit IPv6 wss listen configured in uppercase matches resolved lowercase /tls/ws",
			listenAddrs: mustMultiaddrs(t, "/ip6/fd7d:54ce:fe4::1/tcp/8081/tls/ws"),
			swarmListen: []string{"/ip6/FD7D:54CE:FE4::1/tcp/8081/wss"},
			addrFilters: []string{"/ip6/fc00::/ipcidr/7"},
			want: []deadListenerFinding{
				{Listener: "/ip6/fd7d:54ce:fe4::1/tcp/8081/tls/ws", Rule: "/ip6/fc00::/ipcidr/7", Source: deadListenerSourceAddrFilters, Explicit: true},
			},
		},
		{
			// /tcp/0 binds an OS-assigned port that the config entry cannot
			// name, so the listener cannot be matched back and stays DEBUG.
			name:        "explicit /tcp/0 listen resolves to an assigned port: falls back to non-explicit",
			listenAddrs: mustMultiaddrs(t, "/ip4/127.0.0.1/tcp/54321"),
			swarmListen: []string{"/ip4/127.0.0.1/tcp/0"},
			addrFilters: []string{"/ip4/127.0.0.0/ipcidr/8"},
			want: []deadListenerFinding{
				{Listener: "/ip4/127.0.0.1/tcp/54321", Rule: "/ip4/127.0.0.0/ipcidr/8", Source: deadListenerSourceAddrFilters, Explicit: false},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findDeadListeners(tc.listenAddrs, tc.swarmListen, tc.addrFilters, tc.noAnnounce)
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
