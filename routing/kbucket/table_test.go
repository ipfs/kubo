package kbucket

import (
	crand "crypto/rand"
	"crypto/sha256"
	"math/rand"
	"testing"
	"time"

	tu "github.com/jbenet/go-ipfs/util/testutil"

	peer "github.com/jbenet/go-ipfs/peer"
)

func RandID() ID {
	buf := make([]byte, 16)
	crand.Read(buf)

	hash := sha256.Sum256(buf)
	return ID(hash[:])
}

// Test basic features of the bucket struct
func TestBucket(t *testing.T) {
	b := newBucket()

	peers := make([]peer.Peer, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeer()
		b.pushFront(peers[i])
	}

	local := tu.RandPeer()
	localID := ConvertPeerID(local.ID())

	i := rand.Intn(len(peers))
	e := b.find(peers[i].ID())
	if e == nil {
		t.Errorf("Failed to find peer: %v", peers[i])
	}

	spl := b.Split(0, ConvertPeerID(local.ID()))
	llist := b.list
	for e := llist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(peer.Peer).ID())
		cpl := commonPrefixLen(p, localID)
		if cpl > 0 {
			t.Fatalf("Split failed. found id with cpl > 0 in 0 bucket")
		}
	}

	rlist := spl.list
	for e := rlist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(peer.Peer).ID())
		cpl := commonPrefixLen(p, localID)
		if cpl == 0 {
			t.Fatalf("Split failed. found id with cpl == 0 in non 0 bucket")
		}
	}
}

// Right now, this just makes sure that it doesnt hang or crash
func TestTableUpdate(t *testing.T) {
	local := tu.RandPeer()
	rt := NewRoutingTable(10, ConvertPeerID(local.ID()), time.Hour)

	peers := make([]peer.Peer, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeer()
	}

	// Testing Update
	for i := 0; i < 10000; i++ {
		p := rt.Update(peers[rand.Intn(len(peers))])
		if p != nil {
			//t.Log("evicted peer.")
		}
	}

	for i := 0; i < 100; i++ {
		id := RandID()
		ret := rt.NearestPeers(id, 5)
		if len(ret) == 0 {
			t.Fatal("Failed to find node near ID.")
		}
	}
}

func TestTableFind(t *testing.T) {
	local := tu.RandPeer()
	rt := NewRoutingTable(10, ConvertPeerID(local.ID()), time.Hour)

	peers := make([]peer.Peer, 100)
	for i := 0; i < 5; i++ {
		peers[i] = tu.RandPeer()
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeer(ConvertPeerID(peers[2].ID()))
	if !found.ID().Equal(peers[2].ID()) {
		t.Fatalf("Failed to lookup known node...")
	}
}

func TestTableFindMultiple(t *testing.T) {
	local := tu.RandPeer()
	rt := NewRoutingTable(20, ConvertPeerID(local.ID()), time.Hour)

	peers := make([]peer.Peer, 100)
	for i := 0; i < 18; i++ {
		peers[i] = tu.RandPeer()
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeers(ConvertPeerID(peers[2].ID()), 15)
	if len(found) != 15 {
		t.Fatalf("Got back different number of peers than we expected.")
	}
}

// Looks for race conditions in table operations. For a more 'certain'
// test, increase the loop counter from 1000 to a much higher number
// and set GOMAXPROCS above 1
func TestTableMultithreaded(t *testing.T) {
	local := peer.ID("localPeer")
	tab := NewRoutingTable(20, ConvertPeerID(local), time.Hour)
	var peers []peer.Peer
	for i := 0; i < 500; i++ {
		peers = append(peers, tu.RandPeer())
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
			tab.Find(peers[n].ID())
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
	tab := NewRoutingTable(20, local, time.Hour)

	var peers []peer.Peer
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeer())
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Update(peers[i])
	}
}

func BenchmarkFinds(b *testing.B) {
	b.StopTimer()
	local := ConvertKey("localKey")
	tab := NewRoutingTable(20, local, time.Hour)

	var peers []peer.Peer
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeer())
		tab.Update(peers[i])
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Find(peers[i].ID())
	}
}
