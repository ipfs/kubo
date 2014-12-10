package wantlist

import (
	u "github.com/jbenet/go-ipfs/util"
	"sort"
)

type Wantlist struct {
	set map[u.Key]*Entry
}

func New() *Wantlist {
	return &Wantlist{
		set: make(map[u.Key]*Entry),
	}
}

type Entry struct {
	Value    u.Key
	Priority int
}

func (w *Wantlist) Add(k u.Key, priority int) {
	if _, ok := w.set[k]; ok {
		return
	}
	w.set[k] = &Entry{
		Value:    k,
		Priority: priority,
	}
}

func (w *Wantlist) Remove(k u.Key) {
	delete(w.set, k)
}

func (w *Wantlist) Contains(k u.Key) bool {
	_, ok := w.set[k]
	return ok
}

type entrySlice []*Entry

func (es entrySlice) Len() int           { return len(es) }
func (es entrySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es entrySlice) Less(i, j int) bool { return es[i].Priority > es[j].Priority }

func (w *Wantlist) Entries() []*Entry {
	var es entrySlice

	for _, e := range w.set {
		es = append(es, e)
	}
	sort.Sort(es)
	return es
}
