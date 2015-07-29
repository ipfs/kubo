package key

import (
	"sync"
)

type KeySet interface {
	Add(Key)
	Has(Key) bool
	Remove(Key)
	Keys() []Key
}

type keySet struct {
	keys map[Key]struct{}
}

func NewKeySet() *keySet {
	return &keySet{make(map[Key]struct{})}
}

func (gcs *keySet) Add(k Key) {
	gcs.keys[k] = struct{}{}
}

func (gcs *keySet) Has(k Key) bool {
	_, has := gcs.keys[k]
	return has
}

func (ks *keySet) Keys() []Key {
	var out []Key
	for k, _ := range ks.keys {
		out = append(out, k)
	}
	return out
}

func (ks *keySet) Remove(k Key) {
	delete(ks.keys, k)
}

// TODO: implement disk-backed keyset for working with massive DAGs

type threadsafe struct {
	lk sync.Mutex
	ks KeySet
}

func Threadsafe(ks KeySet) *threadsafe {
	return &threadsafe{ks: ks}
}

func (ts *threadsafe) Has(k Key) bool {
	ts.lk.Lock()
	out := ts.ks.Has(k)
	ts.lk.Unlock() //defer is slow
	return out
}

func (ts *threadsafe) Remove(k Key) {
	ts.lk.Lock()
	ts.ks.Remove(k)
	ts.lk.Unlock() //defer is slow
}

func (ts *threadsafe) Add(k Key) {
	ts.lk.Lock()
	ts.ks.Add(k)
	ts.lk.Unlock() //defer is slow
}

func (ts *threadsafe) Keys() []Key {
	ts.lk.Lock()
	keys := ts.ks.Keys()
	ts.lk.Unlock() //defer is slow
	return keys
}
