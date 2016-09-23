package providers

import (
	"fmt"
	"testing"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestProviderManager(t *testing.T) {
	ctx := context.Background()
	mid := peer.ID("testing")
	p := NewProviderManager(ctx, mid, ds.NewMapDatastore())
	a := key.Key("test")
	p.AddProvider(ctx, a, peer.ID("testingprovider"))
	resp := p.GetProviders(ctx, a)
	if len(resp) != 1 {
		t.Fatal("Could not retrieve provider.")
	}
	p.proc.Close()
}

func TestProvidersDatastore(t *testing.T) {
	old := lruCacheSize
	lruCacheSize = 10
	defer func() { lruCacheSize = old }()

	ctx := context.Background()
	mid := peer.ID("testing")
	p := NewProviderManager(ctx, mid, ds.NewMapDatastore())
	defer p.proc.Close()

	friend := peer.ID("friend")
	var keys []key.Key
	for i := 0; i < 100; i++ {
		k := key.Key(fmt.Sprint(i))
		keys = append(keys, k)
		p.AddProvider(ctx, k, friend)
	}

	for _, k := range keys {
		resp := p.GetProviders(ctx, k)
		if len(resp) != 1 {
			t.Fatal("Could not retrieve provider.")
		}
		if resp[0] != friend {
			t.Fatal("expected provider to be 'friend'")
		}
	}
}

func TestProvidersSerialization(t *testing.T) {
	dstore := ds.NewMapDatastore()

	k := key.Key("my key!")
	p1 := peer.ID("peer one")
	p2 := peer.ID("peer two")
	pt1 := time.Now()
	pt2 := pt1.Add(time.Hour)

	err := writeProviderEntry(dstore, k, p1, pt1)
	if err != nil {
		t.Fatal(err)
	}

	err = writeProviderEntry(dstore, k, p2, pt2)
	if err != nil {
		t.Fatal(err)
	}

	pset, err := loadProvSet(dstore, k)
	if err != nil {
		t.Fatal(err)
	}

	lt1, ok := pset.set[p1]
	if !ok {
		t.Fatal("failed to load set correctly")
	}

	if pt1 != lt1 {
		t.Fatal("time wasnt serialized correctly")
	}

	lt2, ok := pset.set[p2]
	if !ok {
		t.Fatal("failed to load set correctly")
	}

	if pt2 != lt2 {
		t.Fatal("time wasnt serialized correctly")
	}
}

func TestProvidesExpire(t *testing.T) {
	pval := ProvideValidity
	cleanup := defaultCleanupInterval
	ProvideValidity = time.Second / 2
	defaultCleanupInterval = time.Second / 2
	defer func() {
		ProvideValidity = pval
		defaultCleanupInterval = cleanup
	}()

	ctx := context.Background()
	mid := peer.ID("testing")
	p := NewProviderManager(ctx, mid, ds.NewMapDatastore())

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

	time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		out := p.GetProviders(ctx, keys[i])
		if len(out) > 2 {
			t.Fatal("expected providers to be cleaned up")
		}
	}

	if p.providers.Len() != 0 {
		t.Fatal("providers map not cleaned up")
	}

	allprovs, err := p.getAllProvKeys()
	if err != nil {
		t.Fatal(err)
	}

	if len(allprovs) != 0 {
		t.Fatal("expected everything to be cleaned out of the datastore")
	}
}
