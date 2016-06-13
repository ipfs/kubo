package dht

import (
	"testing"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"

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

func TestProvidesExpire(t *testing.T) {
	ProvideValidity = time.Second
	defaultCleanupInterval = time.Second

	ctx := context.Background()
	mid := peer.ID("testing")
	p := NewProviderManager(ctx, mid)

	peers := []peer.ID{"a", "b"}
	var keys []key.Key
	for i := 0; i < 10; i++ {
		k := key.Key(i)
		keys = append(keys, k)
		p.AddProvider(ctx, k, peers[0])
		p.AddProvider(ctx, k, peers[1])
	}

	for i := 0; i < 10; i++ {
		out := p.GetProviders(ctx, keys[i])
		if len(out) != 2 {
			t.Fatal("expected providers to still be there")
		}
	}

	time.Sleep(time.Second * 3)
	for i := 0; i < 10; i++ {
		out := p.GetProviders(ctx, keys[i])
		if len(out) > 2 {
			t.Fatal("expected providers to be cleaned up")
		}
	}

	if len(p.providers) != 0 {
		t.Fatal("providers map not cleaned up")
	}
}
