package peering

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/stretchr/testify/require"
)

func newNode(ctx context.Context, t *testing.T) host.Host {
	h, err := libp2p.New(
		ctx,
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		// We'd like to set the connection manager low water to 0, but
		// that would disable the connection manager.
		libp2p.ConnectionManager(connmgr.NewConnManager(1, 100, 0)),
	)
	require.NoError(t, err)
	return h
}

func TestPeeringService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h1 := newNode(ctx, t)
	ps1 := NewPeeringService(h1)

	h2 := newNode(ctx, t)
	h3 := newNode(ctx, t)
	h4 := newNode(ctx, t)

	// peer 1 -> 2
	ps1.AddPeer(peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})

	// We haven't started so we shouldn't have any peers.
	require.Never(t, func() bool {
		return len(h1.Network().Peers()) > 0
	}, 100*time.Millisecond, 1*time.Second, "expected host 1 to have no peers")

	// Use p4 to take up the one slot we have in the connection manager.
	for _, h := range []host.Host{h1, h2} {
		require.NoError(t, h.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}))
		h.ConnManager().TagPeer(h4.ID(), "sticky-peer", 1000)
	}

	// Now start.
	require.NoError(t, ps1.Start())
	// starting twice is fine.
	require.NoError(t, ps1.Start())

	// We should eventually connect.
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) == network.Connected
	}, 30*time.Second, 10*time.Millisecond)

	// Now explicitly connect to p3.
	require.NoError(t, h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()}))
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) == network.Connected
	}, 30*time.Second, 100*time.Millisecond)

	require.Len(t, h1.Network().Peers(), 3)

	// force a disconnect
	h1.ConnManager().TrimOpenConns(ctx)

	// Should disconnect from p3.
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h3.ID()) != network.Connected
	}, 5*time.Second, 10*time.Millisecond)

	// Should remain connected to p2
	require.Never(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) != network.Connected
	}, 5*time.Second, 1*time.Second)

	// Now force h2 to disconnect (we have an asymmetric peering).
	conns := h2.Network().ConnsToPeer(h1.ID())
	require.NotEmpty(t, conns)
	h2.ConnManager().TrimOpenConns(ctx)

	// All conns to peer should eventually close.
	for _, c := range conns {
		require.Eventually(t, func() bool {
			s, err := c.NewStream(context.Background())
			if s != nil {
				_ = s.Reset()
			}
			return err != nil
		}, 5*time.Second, 10*time.Millisecond)
	}

	// Should eventually re-connect.
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) == network.Connected
	}, 30*time.Second, 1*time.Second)

	// Unprotect 2 from 1.
	ps1.RemovePeer(h2.ID())

	// Trim connections.
	h1.ConnManager().TrimOpenConns(ctx)

	// Should disconnect
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) != network.Connected
	}, 5*time.Second, 10*time.Millisecond)

	// Should never reconnect.
	require.Never(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) == network.Connected
	}, 20*time.Second, 1*time.Second)

	// Until added back
	ps1.AddPeer(peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	ps1.AddPeer(peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()})
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h2.ID()) == network.Connected
	}, 30*time.Second, 1*time.Second)
	require.Eventually(t, func() bool {
		return h1.Network().Connectedness(h3.ID()) == network.Connected
	}, 30*time.Second, 1*time.Second)

	// Should be able to repeatedly stop.
	require.NoError(t, ps1.Stop())
	require.NoError(t, ps1.Stop())

	// Adding and removing should work after stopping.
	ps1.AddPeer(peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()})
	ps1.RemovePeer(h2.ID())
}

func TestNextBackoff(t *testing.T) {
	minMaxBackoff := (100 - maxBackoffJitter) / 100 * maxBackoff
	for x := 0; x < 1000; x++ {
		ph := peerHandler{nextDelay: time.Second}
		for min, max := time.Second*3/2, time.Second*5/2; min < minMaxBackoff; min, max = min*3/2, max*5/2 {
			b := ph.nextBackoff()
			if b > max || b < min {
				t.Errorf("expected backoff %s to be between %s and %s", b, min, max)
			}
		}
		for i := 0; i < 100; i++ {
			b := ph.nextBackoff()
			if b < minMaxBackoff || b > maxBackoff {
				t.Fatal("failed to stay within max bounds")
			}
		}
	}
}
