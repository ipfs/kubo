package core

import (
	"fmt"
	"testing"

	pstore "gx/ipfs/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	testutil "gx/ipfs/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
	config "gx/ipfs/QmbK4EmM2Xx5fmbqK38TGP3PpY66r3tkXLZTcc7dF9mFwM/go-ipfs-config"
)

func TestSubsetWhenMaxIsGreaterThanLengthOfSlice(t *testing.T) {
	var ps []pstore.PeerInfo
	sizeofSlice := 100
	for i := 0; i < sizeofSlice; i++ {
		pid, err := testutil.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		ps = append(ps, pstore.PeerInfo{ID: pid})
	}
	out := randomSubsetOfPeers(ps, 2*sizeofSlice)
	if len(out) != len(ps) {
		t.Fail()
	}
}

func TestMultipleAddrsPerPeer(t *testing.T) {
	var bsps []config.BootstrapPeer
	for i := 0; i < 10; i++ {
		pid, err := testutil.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/5001/ipfs/%s", pid.Pretty())
		bsp1, err := config.ParseBootstrapPeer(addr)
		if err != nil {
			t.Fatal(err)
		}

		addr = fmt.Sprintf("/ip4/127.0.0.1/udp/5002/utp/ipfs/%s", pid.Pretty())
		bsp2, err := config.ParseBootstrapPeer(addr)
		if err != nil {
			t.Fatal(err)
		}

		bsps = append(bsps, bsp1, bsp2)
	}

	pinfos := toPeerInfos(bsps)
	if len(pinfos) != len(bsps)/2 {
		t.Fatal("expected fewer peers")
	}
}
