package bootstrap

import (
	"context"
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

func TestLoadAndSaveOptions(t *testing.T) {
	loadFunc := func(_ context.Context) []peer.AddrInfo { return nil }
	saveFunc := func(_ context.Context, _ []peer.AddrInfo) {}

	bootCfg := BootstrapConfigWithPeers(nil, WithBackupPeers(loadFunc, saveFunc))
	if bootCfg.LoadBackupBootstrapPeers == nil {
		t.Fatal("load function not assigned")
	}
	if bootCfg.SaveBackupBootstrapPeers == nil {
		t.Fatal("save function not assigned")
	}

	assertPanics(t, "with only load func", func() {
		BootstrapConfigWithPeers(nil, WithBackupPeers(loadFunc, nil))
	})

	assertPanics(t, "with only save func", func() {
		BootstrapConfigWithPeers(nil, WithBackupPeers(nil, saveFunc))
	})

	bootCfg = BootstrapConfigWithPeers(nil, WithBackupPeers(nil, nil))
	if bootCfg.LoadBackupBootstrapPeers != nil || bootCfg.SaveBackupBootstrapPeers != nil {
		t.Fatal("load and save functions should both be nil")
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

	// Test for error is only Load or Save function defined.
	bootCfg.LoadBackupBootstrapPeers = func(_ context.Context) []peer.AddrInfo { return nil }

	_, err = Bootstrap(peerID, p2pHost, nil, bootCfg)
	if err == nil {
		t.Fatal("expected error")
	}

	bootCfg.LoadBackupBootstrapPeers = nil
	bootCfg.SaveBackupBootstrapPeers = func(_ context.Context, _ []peer.AddrInfo) {}

	_, err = Bootstrap(peerID, p2pHost, nil, bootCfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func assertPanics(t *testing.T, name string, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: did not panic as expected", name)
		}
	}()

	f()
}
