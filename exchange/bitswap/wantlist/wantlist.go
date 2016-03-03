// package wantlist implements an object for bitswap that contains the keys
// that a given peer wants.
package wantlist

import (
	"sort"
	"sync"

	key "github.com/ipfs/go-ipfs/blocks/key"

	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type ThreadSafe struct {
	lk       sync.RWMutex
	Wantlist Wantlist
}

// not threadsafe
type Wantlist struct {
	set map[key.Key]Entry
}

type Entry struct {
	// TODO consider making entries immutable so they can be shared safely and
	// slices can be copied efficiently.
	Key      key.Key
	Priority int
	Ctx      context.Context
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
		set: make(map[key.Key]Entry),
	}
}

func (w *ThreadSafe) Add(ctx context.Context, k key.Key, priority int) {
	w.lk.Lock()
	defer w.lk.Unlock()
	w.Wantlist.Add(ctx, k, priority)
}

func (w *ThreadSafe) AddEntry(e Entry) {
	w.lk.Lock()
	defer w.lk.Unlock()
	w.Wantlist.AddEntry(e)
}

func (w *ThreadSafe) Remove(k key.Key) {
	w.lk.Lock()
	defer w.lk.Unlock()
	w.Wantlist.Remove(k)
}

func (w *ThreadSafe) Contains(k key.Key) (Entry, bool) {
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

func (w *Wantlist) Add(ctx context.Context, k key.Key, priority int) {
	if _, ok := w.set[k]; ok {
		return
	}
	w.set[k] = Entry{
		Key:      k,
		Priority: priority,
		Ctx:      ctx,
	}
}

func (w *Wantlist) AddEntry(e Entry) {
	if _, ok := w.set[e.Key]; ok {
		return
	}
	w.set[e.Key] = e
}

func (w *Wantlist) Remove(k key.Key) {
	delete(w.set, k)
}

func (w *Wantlist) Contains(k key.Key) (Entry, bool) {
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
