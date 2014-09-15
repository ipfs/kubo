package dht

import (
	"testing"

	"github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

func TestProviderManager(t *testing.T) {
	mid := peer.ID("testing")
	p := NewProviderManager(mid)
	a := u.Key("test")
	p.AddProvider(a, &peer.Peer{})
	remotePeers := p.GetProviders(a)
	localPeers := p.GetLocal()
	if len(remotePeers) != 1 {
		t.Fatal("Could not retrieve remote provider.")
	}
	if len(localPeers) != 1 {
		t.Fatal("Could not retrieve local provider.")
	}

	p.Halt()
}
