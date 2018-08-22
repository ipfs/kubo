package core

import (
	"fmt"
	"testing"

	testutil "gx/ipfs/QmRNhSdqzMcuRxX9A1egBeQ3BhDTguDV5HPwi8wRykkPU8/go-testutil"
	config "gx/ipfs/QmTyiSs9VgdVb4pnzdjtKhcfdTkHFEaNn6xnCbZq4DTFRt/go-ipfs-config"
	pstore "gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"
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
