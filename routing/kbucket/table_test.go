package kbucket

import (
	"math/rand"
	"testing"
	"time"

	tu "github.com/ipfs/go-ipfs/thirdparty/testutil"

	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
)

// Test basic features of the bucket struct
func TestBucket(t *testing.T) {
	b := newBucket()

	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		b.PushFront(peers[i])
	}

	local := tu.RandPeerIDFatal(t)
	localID := ConvertPeerID(local)

	i := rand.Intn(len(peers))
	if !b.Has(peers[i]) {
		t.Errorf("Failed to find peer: %v", peers[i])
	}

	spl := b.Split(0, ConvertPeerID(local))
	llist := b.list
	for e := llist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(peer.ID))
		cpl := commonPrefixLen(p, localID)
		if cpl > 0 {
			t.Fatalf("Split failed. found id with cpl > 0 in 0 bucket")
		}
	}

	rlist := spl.list
	for e := rlist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(peer.ID))
		cpl := commonPrefixLen(p, localID)
		if cpl == 0 {
			t.Fatalf("Split failed. found id with cpl == 0 in non 0 bucket")
		}
	}
}

// Right now, this just makes sure that it doesnt hang or crash
func TestTableUpdate(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := peer.NewMetrics()
	rt := NewRoutingTable(10, ConvertPeerID(local), time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
	}

	// Testing Update
	for i := 0; i < 10000; i++ {
		rt.Update(peers[rand.Intn(len(peers))])
	}

	for i := 0; i < 100; i++ {
		id := ConvertPeerID(tu.RandPeerIDFatal(t))
		ret := rt.NearestPeers(id, 5)
		if len(ret) == 0 {
			t.Fatal("Failed to find node near ID.")
		}
	}
}

func TestTableFind(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := peer.NewMetrics()
	rt := NewRoutingTable(10, ConvertPeerID(local), time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 5; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeer(ConvertPeerID(peers[2]))
	if !(found == peers[2]) {
		t.Fatalf("Failed to lookup known node...")
	}
}

func TestTableFindMultiple(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := peer.NewMetrics()
	rt := NewRoutingTable(20, ConvertPeerID(local), time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 18; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeers(ConvertPeerID(peers[2]), 15)
	if len(found) != 15 {
		t.Fatalf("Got back different number of peers than we expected.")
	}
}

// Looks for race conditions in table operations. For a more 'certain'
// test, increase the loop counter from 1000 to a much higher number
// and set GOMAXPROCS above 1
func TestTableMultithreaded(t *testing.T) {
	local := peer.ID("localPeer")
	m := peer.NewMetrics()
	tab := NewRoutingTable(20, ConvertPeerID(local), time.Hour, m)
	var peers []peer.ID
	for i := 0; i < 500; i++ {
		peers = append(peers, tu.RandPeerIDFatal(t))
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			n := rand.Intn(len(peers))
			tab.Update(peers[n])
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			n := rand.Intn(len(peers))
			tab.Update(peers[n])
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			n := rand.Intn(len(peers))
			tab.Find(peers[n])
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	<-done
}

func BenchmarkUpdates(b *testing.B) {
	b.StopTimer()
	local := ConvertKey("localKey")
	m := peer.NewMetrics()
	tab := NewRoutingTable(20, local, time.Hour, m)

	var peers []peer.ID
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeerIDFatal(b))
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Update(peers[i])
	}
}

func BenchmarkFinds(b *testing.B) {
	b.StopTimer()
	local := ConvertKey("localKey")
	m := peer.NewMetrics()
	tab := NewRoutingTable(20, local, time.Hour, m)

	var peers []peer.ID
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeerIDFatal(b))
		tab.Update(peers[i])
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Find(peers[i])
	}
}
