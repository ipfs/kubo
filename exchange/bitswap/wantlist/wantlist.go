package wantlist

import (
	u "github.com/jbenet/go-ipfs/util"
	"sort"
	"sync"
)

type Wantlist struct {
	lk  sync.RWMutex
	set map[u.Key]*Entry
}

func New() *Wantlist {
	return &Wantlist{
		set: make(map[u.Key]*Entry),
	}
}

type Entry struct {
	Key      u.Key
	Priority int
}

func (w *Wantlist) Add(k u.Key, priority int) {
	w.lk.Lock()
	defer w.lk.Unlock()
	if _, ok := w.set[k]; ok {
		return
	}
	w.set[k] = &Entry{
		Key:      k,
		Priority: priority,
	}
}

func (w *Wantlist) Remove(k u.Key) {
	w.lk.Lock()
	defer w.lk.Unlock()
	delete(w.set, k)
}

func (w *Wantlist) Contains(k u.Key) bool {
	w.lk.RLock()
	defer w.lk.RUnlock()
	_, ok := w.set[k]
	return ok
}

type entrySlice []*Entry

func (es entrySlice) Len() int           { return len(es) }
func (es entrySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es entrySlice) Less(i, j int) bool { return es[i].Priority > es[j].Priority }

func (w *Wantlist) Entries() []*Entry {
	w.lk.RLock()
	defer w.lk.RUnlock()

	var es entrySlice

	for _, e := range w.set {
		es = append(es, e)
	}
	sort.Sort(es)
	return es
}

func (w *Wantlist) SortedEntries() []*Entry {
	w.lk.RLock()
	defer w.lk.RUnlock()
	var es entrySlice

	for _, e := range w.set {
		es = append(es, e)
	}
	sort.Sort(es)
	return es
}
