package dht

import (
	"container/list"
	crand "crypto/rand"
	"crypto/sha256"
	"math/rand"
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
)

func _randPeer() *peer.Peer {
	p := new(peer.Peer)
	p.ID = make(peer.ID, 16)
	crand.Read(p.ID)
	return p
}

func _randID() ID {
	buf := make([]byte, 16)
	crand.Read(buf)

	hash := sha256.Sum256(buf)
	return ID(hash[:])
}

// Test basic features of the bucket struct
func TestBucket(t *testing.T) {
	b := new(Bucket)

	peers := make([]*peer.Peer, 100)
	for i := 0; i < 100; i++ {
		peers[i] = _randPeer()
		b.PushFront(peers[i])
	}

	local := _randPeer()
	local_id := ConvertPeerID(local.ID)

	i := rand.Intn(len(peers))
	e := b.Find(peers[i].ID)
	if e == nil {
		t.Errorf("Failed to find peer: %v", peers[i])
	}

	spl := b.Split(0, ConvertPeerID(local.ID))
	llist := (*list.List)(b)
	for e := llist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(*peer.Peer).ID)
		cpl := xor(p, local_id).commonPrefixLen()
		if cpl > 0 {
			t.Fatalf("Split failed. found id with cpl > 0 in 0 bucket")
		}
	}

	rlist := (*list.List)(spl)
	for e := rlist.Front(); e != nil; e = e.Next() {
		p := ConvertPeerID(e.Value.(*peer.Peer).ID)
		cpl := xor(p, local_id).commonPrefixLen()
		if cpl == 0 {
			t.Fatalf("Split failed. found id with cpl == 0 in non 0 bucket")
		}
	}
}

// Right now, this just makes sure that it doesnt hang or crash
func TestTableUpdate(t *testing.T) {
	local := _randPeer()
	rt := NewRoutingTable(10, ConvertPeerID(local.ID))

	peers := make([]*peer.Peer, 100)
	for i := 0; i < 100; i++ {
		peers[i] = _randPeer()
	}

	// Testing Update
	for i := 0; i < 10000; i++ {
		p := rt.Update(peers[rand.Intn(len(peers))])
		if p != nil {
			t.Log("evicted peer.")
		}
	}

	for i := 0; i < 100; i++ {
		id := _randID()
		ret := rt.NearestPeers(id, 5)
		if len(ret) == 0 {
			t.Fatal("Failed to find node near ID.")
		}
	}
}

func TestTableFind(t *testing.T) {
	local := _randPeer()
	rt := NewRoutingTable(10, ConvertPeerID(local.ID))

	peers := make([]*peer.Peer, 100)
	for i := 0; i < 5; i++ {
		peers[i] = _randPeer()
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2].ID.Pretty())
	found := rt.NearestPeer(ConvertPeerID(peers[2].ID))
	if !found.ID.Equal(peers[2].ID) {
		t.Fatalf("Failed to lookup known node...")
	}
}

func TestTableFindMultiple(t *testing.T) {
	local := _randPeer()
	rt := NewRoutingTable(20, ConvertPeerID(local.ID))

	peers := make([]*peer.Peer, 100)
	for i := 0; i < 18; i++ {
		peers[i] = _randPeer()
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2].ID.Pretty())
	found := rt.NearestPeers(ConvertPeerID(peers[2].ID), 15)
	if len(found) != 15 {
		t.Fatalf("Got back different number of peers than we expected.")
	}
}
