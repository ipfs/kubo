package dht

import (
	"container/list"

	peer "github.com/jbenet/go-ipfs/peer"
)
// Bucket holds a list of peers.
type Bucket list.List

func (b *Bucket) Find(id peer.ID) *list.Element {
	bucket_list := (*list.List)(b)
	for e := bucket_list.Front(); e != nil; e = e.Next() {
		if e.Value.(*peer.Peer).ID.Equal(id) {
			return e
		}
	}
	return nil
}

func (b *Bucket) MoveToFront(e *list.Element) {
	bucket_list := (*list.List)(b)
	bucket_list.MoveToFront(e)
}

func (b *Bucket) PushFront(p *peer.Peer) {
	bucket_list := (*list.List)(b)
	bucket_list.PushFront(p)
}

func (b *Bucket) PopBack() *peer.Peer {
	bucket_list := (*list.List)(b)
	last := bucket_list.Back()
	bucket_list.Remove(last)
	return last.Value.(*peer.Peer)
}

func (b *Bucket) Len() int {
	bucket_list := (*list.List)(b)
	return bucket_list.Len()
}

// Splits a buckets peers into two buckets, the methods receiver will have
// peers with CPL equal to cpl, the returned bucket will have peers with CPL
// greater than cpl (returned bucket has closer peers)
func (b *Bucket) Split(cpl int, target ID) *Bucket {
	bucket_list := (*list.List)(b)
	out := list.New()
	e := bucket_list.Front()
	for e != nil {
		peer_id := ConvertPeerID(e.Value.(*peer.Peer).ID)
		peer_cpl := prefLen(peer_id, target)
		if peer_cpl > cpl {
			cur := e
			out.PushBack(e.Value)
			e = e.Next()
			bucket_list.Remove(cur)
			continue
		}
		e = e.Next()
	}
	return (*Bucket)(out)
}

func (b *Bucket) getIter() *list.Element {
	bucket_list := (*list.List)(b)
	return bucket_list.Front()
}
