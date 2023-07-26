package bootstrap

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/test"
)

func TestRandomizeAddressList(t *testing.T) {
	var ps []peer.AddrInfo
	sizeofSlice := 10
	for i := 0; i < sizeofSlice; i++ {
		pid, err := test.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		ps = append(ps, peer.AddrInfo{ID: pid})
	}
	out := randomizeList(ps)
	if len(out) != len(ps) {
		t.Fail()
	}
}

func TestNoTempPeersLoadAndSave(t *testing.T) {
	period := 500 * time.Millisecond
	bootCfg := BootstrapConfigWithPeers(nil)
	bootCfg.MinPeerThreshold = 2
	bootCfg.Period = period

	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	p2pHost, err := libp2p.New(libp2p.Identity(priv))
	if err != nil {
		t.Fatal(err)
	}

	bootstrapper, err := Bootstrap(peerID, p2pHost, nil, bootCfg)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(4 * period)
	bootstrapper.Close()
}
