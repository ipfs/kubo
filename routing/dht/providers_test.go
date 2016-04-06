package dht

import (
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	peer "gx/ipfs/QmZMehXD2w81qeVJP6r1mmocxwsD7kqAvuzGm2QWDw1H88/go-libp2p/p2p/peer"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestProviderManager(t *testing.T) {
	ctx := context.Background()
	mid := peer.ID("testing")
	p := NewProviderManager(ctx, mid)
	a := key.Key("test")
	p.AddProvider(ctx, a, peer.ID("testingprovider"))
	resp := p.GetProviders(ctx, a)
	if len(resp) != 1 {
		t.Fatal("Could not retrieve provider.")
	}
	p.proc.Close()
}
