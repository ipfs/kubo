package dht

import (
	"container/list"
	"sync"

	peer "github.com/jbenet/go-ipfs/peer"
)

// Bucket holds a list of peers.
type Bucket struct {
	lk   sync.RWMutex
	list *list.List
}

func NewBucket() *Bucket {
	b := new(Bucket)
	b.list = list.New()
	return b
}

func (b *Bucket) Find(id peer.ID) *list.Element {
	b.lk.RLock()
	defer b.lk.RUnlock()
	for e := b.list.Front(); e != nil; e = e.Next() {
		if e.Value.(*peer.Peer).ID.Equal(id) {
			return e
		}
	}
	return nil
}

func (b *Bucket) MoveToFront(e *list.Element) {
	b.lk.Lock()
	b.list.MoveToFront(e)
	b.lk.Unlock()
}

func (b *Bucket) PushFront(p *peer.Peer) {
	b.lk.Lock()
	b.list.PushFront(p)
	b.lk.Unlock()
}

func (b *Bucket) PopBack() *peer.Peer {
	b.lk.Lock()
	defer b.lk.Unlock()
	last := b.list.Back()
	b.list.Remove(last)
	return last.Value.(*peer.Peer)
}

func (b *Bucket) Len() int {
	b.lk.RLock()
	defer b.lk.RUnlock()
	return b.list.Len()
}

// Splits a buckets peers into two buckets, the methods receiver will have
// peers with CPL equal to cpl, the returned bucket will have peers with CPL
// greater than cpl (returned bucket has closer peers)
func (b *Bucket) Split(cpl int, target ID) *Bucket {
	b.lk.Lock()
	defer b.lk.Unlock()

	out := list.New()
	newbuck := NewBucket()
	newbuck.list = out
	e := b.list.Front()
	for e != nil {
		peer_id := ConvertPeerID(e.Value.(*peer.Peer).ID)
		peer_cpl := prefLen(peer_id, target)
		if peer_cpl > cpl {
			cur := e
			out.PushBack(e.Value)
			e = e.Next()
			b.list.Remove(cur)
			continue
		}
		e = e.Next()
	}
	return newbuck
}

func (b *Bucket) getIter() *list.Element {
	return b.list.Front()
}
