package bootstrap

import (
	"fmt"
	"testing"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/test"
)

func TestSubsetWhenMaxIsGreaterThanLengthOfSlice(t *testing.T) {
	var ps []peer.AddrInfo
	sizeofSlice := 100
	for i := 0; i < sizeofSlice; i++ {
		pid, err := test.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		ps = append(ps, peer.AddrInfo{ID: pid})
	}
	out := randomSubsetOfPeers(ps, 2*sizeofSlice)
	if len(out) != len(ps) {
		t.Fail()
	}
}

func TestMultipleAddrsPerPeer(t *testing.T) {
	var bsps []config.BootstrapPeer
	for i := 0; i < 10; i++ {
		pid, err := test.RandPeerID()
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

	pinfos := Peers.ToPeerInfos(bsps)
	if len(pinfos) != len(bsps)/2 {
		t.Fatal("expected fewer peers")
	}
}
