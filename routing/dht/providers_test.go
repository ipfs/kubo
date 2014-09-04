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
	resp := p.GetProviders(a)
	if len(resp) != 1 {
		t.Fatal("Could not retrieve provider.")
	}
	p.Halt()
}
