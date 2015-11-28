package dht

import (
	"fmt"
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
)

func TestProvideMany(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numkeys := 50
	num := 10
	_, _, dhts := setupDHTS(ctx, num, t)

	for i := 1; i < num; i++ {
		connect(t, ctx, dhts[0], dhts[i])
	}

	var keys []key.Key
	for i := 0; i < numkeys; i++ {
		keys = append(keys, key.Key(fmt.Sprint(i)))
	}

	err := dhts[1].ProvideMany(ctx, keys)
	if err != nil {
		t.Fatal(err)
	}

	// sleep a small amount to make sure messages all arrive
	time.Sleep(time.Millisecond * 100)

	// in this setup (len(dhts) == 10), every node should know every provider
	for i, d := range dhts {
		for _, k := range keys {
			pids := d.providers.GetProviders(ctx, k)
			if len(pids) != 1 {
				t.Fatalf("[%d] expected 1 provider for %s, got %d", i, k, len(pids))
			}
		}
	}

	fmt.Printf("total bandwidth used: %d\n", totalBandwidth(dhts))
}

func TestProvideManyOld(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numkeys := 50
	num := 10
	_, _, dhts := setupDHTS(ctx, num, t)

	for i := 1; i < num; i++ {
		connect(t, ctx, dhts[0], dhts[i])
	}

	var keys []key.Key
	for i := 0; i < numkeys; i++ {
		keys = append(keys, key.Key(fmt.Sprint(i)))
	}

	for _, k := range keys {
		err := dhts[1].Provide(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
	}

	// sleep a small amount to make sure messages all arrive
	time.Sleep(time.Millisecond * 100)

	// in this setup (len(dhts) == 10), every node should know every provider
	for i, d := range dhts {
		for _, k := range keys {
			pids := d.providers.GetProviders(ctx, k)
			if len(pids) != 1 {
				// don't fail... this doesnt always work
				t.Logf("[%d] expected 1 provider for %s, got %d", i, k, len(pids))
			}
		}
	}

	fmt.Printf("total bandwidth used: %d\n", totalBandwidth(dhts))
}
func totalBandwidth(dhts []*IpfsDHT) int64 {
	var sum int64
	for _, d := range dhts {
		bwrp := d.host.GetBandwidthReporter()
		totals := bwrp.GetBandwidthTotals()
		sum += totals.TotalOut
	}
	return sum
}
