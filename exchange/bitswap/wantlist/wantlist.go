package wantlist

import (
	u "github.com/jbenet/go-ipfs/util"
	"sort"
	"sync"
)

type ThreadSafe struct {
	lk       sync.RWMutex
	Wantlist Wantlist
}

// not threadsafe
type Wantlist struct {
	set map[u.Key]Entry
	// TODO provide O(1) len accessor if cost becomes an issue
}

type Entry struct {
	// TODO consider making entries immutable so they can be shared safely and
	// slices can be copied efficiently.
	Key      u.Key
	Priority int
}

type entrySlice []Entry

func (es entrySlice) Len() int           { return len(es) }
func (es entrySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es entrySlice) Less(i, j int) bool { return es[i].Priority > es[j].Priority }

func NewThreadSafe() *ThreadSafe {
	return &ThreadSafe{
		Wantlist: *New(),
	}
}

func New() *Wantlist {
	return &Wantlist{
		set: make(map[u.Key]Entry),
	}
}

func (w *ThreadSafe) Add(k u.Key, priority int) {
	// TODO rm defer for perf
	w.lk.Lock()
	defer w.lk.Unlock()
	w.Wantlist.Add(k, priority)
}

func (w *ThreadSafe) Remove(k u.Key) {
	// TODO rm defer for perf
	w.lk.Lock()
	defer w.lk.Unlock()
	w.Wantlist.Remove(k)
}

func (w *ThreadSafe) Contains(k u.Key) (Entry, bool) {
	// TODO rm defer for perf
	w.lk.RLock()
	defer w.lk.RUnlock()
	return w.Wantlist.Contains(k)
}

func (w *ThreadSafe) Entries() []Entry {
	w.lk.RLock()
	defer w.lk.RUnlock()
	return w.Wantlist.Entries()
}

func (w *ThreadSafe) SortedEntries() []Entry {
	w.lk.RLock()
	defer w.lk.RUnlock()
	return w.Wantlist.SortedEntries()
}

func (w *ThreadSafe) Len() int {
	w.lk.RLock()
	defer w.lk.RUnlock()
	return w.Wantlist.Len()
}

func (w *Wantlist) Len() int {
	return len(w.set)
}

func (w *Wantlist) Add(k u.Key, priority int) {
	if _, ok := w.set[k]; ok {
		return
	}
	w.set[k] = Entry{
		Key:      k,
		Priority: priority,
	}
}

func (w *Wantlist) Remove(k u.Key) {
	delete(w.set, k)
}

func (w *Wantlist) Contains(k u.Key) (Entry, bool) {
	e, ok := w.set[k]
	return e, ok
}

func (w *Wantlist) Entries() []Entry {
	var es entrySlice
	for _, e := range w.set {
		es = append(es, e)
	}
	return es
}

func (w *Wantlist) SortedEntries() []Entry {
	var es entrySlice
	for _, e := range w.set {
		es = append(es, e)
	}
	sort.Sort(es)
	return es
}
