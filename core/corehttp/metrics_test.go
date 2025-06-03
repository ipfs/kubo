package corehttp

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/kubo/core"

	inet "github.com/libp2p/go-libp2p/core/network"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
)

// This test is based on go-libp2p/p2p/net/swarm.TestConnectednessCorrect
// It builds 4 nodes and connects them, one being the sole center.
// Then it checks that the center reports the correct number of peers.
func TestPeersTotal(t *testing.T) {
	ctx := context.Background()

	hosts := make([]*bhost.BasicHost, 4)
	for i := 0; i < 4; i++ {
		var err error
		hosts[i], err = bhost.NewHost(swarmt.GenSwarm(t), nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	dial := func(a, b inet.Network) {
		swarmt.DivulgeAddresses(b, a)
		if _, err := a.DialPeer(ctx, b.LocalPeer()); err != nil {
			t.Fatalf("Failed to dial: %s", err)
		}
	}

	dial(hosts[0].Network(), hosts[1].Network())
	dial(hosts[0].Network(), hosts[2].Network())
	dial(hosts[0].Network(), hosts[3].Network())

	// there's something wrong with dial, i think. it's not finishing
	// completely. there must be some async stuff.
	<-time.After(100 * time.Millisecond)

	node := &core.IpfsNode{PeerHost: hosts[0]}
	collector := IpfsNodeCollector{Node: node}
	peersTransport := collector.PeersTotalValues()
	if len(peersTransport) > 2 {
		t.Fatalf("expected at most 2 peers transport (tcp and upd/quic), got %d, transport map %v",
			len(peersTransport), peersTransport)
	}
	totalPeers := peersTransport["/ip4/tcp"] + peersTransport["/ip4/udp/quic-v1"]
	if totalPeers != 3 {
		t.Fatalf("expected 3 peers in either tcp or upd/quic transport, got %f", totalPeers)
	}
}
