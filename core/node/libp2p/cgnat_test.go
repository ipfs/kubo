package libp2p

import (
	"bytes"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

func mustMA(t *testing.T, ss ...string) []ma.Multiaddr {
	t.Helper()
	out := make([]ma.Multiaddr, len(ss))
	for i, s := range ss {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Fatalf("bad multiaddr %q: %s", s, err)
		}
		out[i] = m
	}
	return out
}

func ipSet(ips ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		set[ip] = struct{}{}
	}
	return set
}

func TestClassifyNAT(t *testing.T) {
	cases := []struct {
		name  string
		all   []ma.Multiaddr
		iface map[string]struct{}
		reach network.Reachability
		want  natKind
	}{
		{
			name:  "cgnat as foreign mapped wan behind private home router",
			all:   mustMA(t, "/ip4/192.168.1.5/tcp/4001", "/ip4/100.64.0.7/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPrivate,
			want:  natCGNAT,
		},
		{
			name:  "double nat: foreign rfc1918 mapped wan",
			all:   mustMA(t, "/ip4/192.168.1.5/tcp/4001", "/ip4/10.20.30.1/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPrivate,
			want:  natDoubleNAT,
		},
		{
			name:  "cgnat wins over double-nat in one snapshot (rfc1918 first)",
			all:   mustMA(t, "/ip4/10.20.30.1/tcp/4001", "/ip4/100.64.0.7/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPrivate,
			want:  natCGNAT,
		},
		{
			name:  "cgnat wins over double-nat in one snapshot (cgnat first)",
			all:   mustMA(t, "/ip4/100.64.0.7/tcp/4001", "/ip4/10.20.30.1/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPrivate,
			want:  natCGNAT,
		},
		{
			name:  "tailscale on local interface not flagged (behind NAT)",
			all:   mustMA(t, "/ip4/192.168.1.5/tcp/4001", "/ip4/100.101.102.103/tcp/4001"),
			iface: ipSet("192.168.1.5", "100.101.102.103"),
			reach: network.ReachabilityPrivate,
			want:  natUnknown,
		},
		{
			name:  "tailscale on local interface not flagged (publicly reachable)",
			all:   mustMA(t, "/ip4/203.0.113.7/tcp/4001", "/ip4/100.101.102.103/tcp/4001"),
			iface: ipSet("203.0.113.7", "100.101.102.103"),
			reach: network.ReachabilityPublic,
			want:  natUnknown,
		},
		{
			name:  "public reachability suppresses foreign cgnat",
			all:   mustMA(t, "/ip4/100.64.0.7/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPublic,
			want:  natUnknown,
		},
		{
			name:  "public reachability suppresses foreign rfc1918",
			all:   mustMA(t, "/ip4/10.20.30.1/tcp/4001"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityPublic,
			want:  natUnknown,
		},
		{
			name:  "ipv6 addresses ignored",
			all:   mustMA(t, "/ip6/fd00::1/tcp/4001", "/ip6/2604:2dc0::1/tcp/4001"),
			iface: ipSet(),
			reach: network.ReachabilityPrivate,
			want:  natUnknown,
		},
		{
			name:  "own private interface across transports not flagged",
			all:   mustMA(t, "/ip4/192.168.1.5/tcp/4001", "/ip4/192.168.1.5/udp/4001/quic-v1"),
			iface: ipSet("192.168.1.5"),
			reach: network.ReachabilityUnknown,
			want:  natUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyNAT(tc.all, tc.iface, tc.reach); got != tc.want {
				t.Fatalf("classifyNAT = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLatchNotice(t *testing.T) {
	cases := []struct {
		name     string
		reported natKind
		detected natKind
		wantKind natKind
		wantEmit bool
	}{
		{"first cgnat", natUnknown, natCGNAT, natCGNAT, true},
		{"first double-nat", natUnknown, natDoubleNAT, natDoubleNAT, true},
		{"upgrade double-nat to cgnat", natDoubleNAT, natCGNAT, natCGNAT, true},
		{"no downgrade cgnat to double-nat", natCGNAT, natDoubleNAT, natCGNAT, false},
		{"repeat cgnat suppressed", natCGNAT, natCGNAT, natCGNAT, false},
		{"repeat double-nat suppressed", natDoubleNAT, natDoubleNAT, natDoubleNAT, false},
		{"unknown detection keeps state", natCGNAT, natUnknown, natCGNAT, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKind, gotEmit := latchNotice(tc.reported, tc.detected)
			if gotKind != tc.wantKind || gotEmit != tc.wantEmit {
				t.Fatalf("latchNotice(%q,%q) = (%q,%v), want (%q,%v)",
					tc.reported, tc.detected, gotKind, gotEmit, tc.wantKind, tc.wantEmit)
			}
		})
	}
}

func TestCGNATNoticeOutput(t *testing.T) {
	t.Run("cgnat", func(t *testing.T) {
		out := captureNotice(t, logCGNATNotice)
		for _, want := range []string{
			"carrier-grade NAT (CGNAT) detected",
			"100.64.0.0/10",
			"Internal.CGNATCheck false",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("notice missing %q; got:\n%s", want, out)
			}
		}
	})
	t.Run("double-nat", func(t *testing.T) {
		out := captureNotice(t, logDoubleNATNotice)
		for _, want := range []string{
			"double NAT",
			"Internal.CGNATCheck false",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("notice missing %q; got:\n%s", want, out)
			}
		}
	})
}

func captureNotice(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	old := cgnatNoticeOut
	cgnatNoticeOut = &buf
	t.Cleanup(func() { cgnatNoticeOut = old })
	fn()
	return buf.String()
}
