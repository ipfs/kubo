package decision

import (
	"sync"
	"time"

	pq "github.com/jbenet/go-ipfs/exchange/bitswap/decision/pq"
	wantlist "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type peerRequestQueue interface {
	// Pop returns the next peerRequestTask. Returns nil if the peerRequestQueue is empty.
	Pop() *peerRequestTask
	Push(entry wantlist.Entry, to peer.ID)
	Remove(k u.Key, p peer.ID)
	// NB: cannot expose simply expose taskQueue.Len because trashed elements
	// may exist. These trashed elements should not contribute to the count.
}

func newPRQ() peerRequestQueue {
	return &prq{
		taskMap:   make(map[string]*peerRequestTask),
		taskQueue: pq.New(wrapCmp(V1)),
	}
}

var _ peerRequestQueue = &prq{}

// TODO: at some point, the strategy needs to plug in here
// to help decide how to sort tasks (on add) and how to select
// tasks (on getnext). For now, we are assuming a dumb/nice strategy.
type prq struct {
	lock      sync.Mutex
	taskQueue pq.PQ
	taskMap   map[string]*peerRequestTask
}

// Push currently adds a new peerRequestTask to the end of the list
func (tl *prq) Push(entry wantlist.Entry, to peer.ID) {
	tl.lock.Lock()
	defer tl.lock.Unlock()
	if task, ok := tl.taskMap[taskKey(to, entry.Key)]; ok {
		task.Entry.Priority = entry.Priority
		tl.taskQueue.Update(task.index)
		return
	}
	task := &peerRequestTask{
		Entry:   entry,
		Target:  to,
		created: time.Now(),
	}
	tl.taskQueue.Push(task)
	tl.taskMap[task.Key()] = task
}

// Pop 'pops' the next task to be performed. Returns nil if no task exists.
func (tl *prq) Pop() *peerRequestTask {
	tl.lock.Lock()
	defer tl.lock.Unlock()
	var out *peerRequestTask
	for tl.taskQueue.Len() > 0 {
		out = tl.taskQueue.Pop().(*peerRequestTask)
		delete(tl.taskMap, out.Key())
		if out.trash {
			continue // discarding tasks that have been removed
		}
		break // and return |out|
	}
	return out
}

// Remove removes a task from the queue
func (tl *prq) Remove(k u.Key, p peer.ID) {
	tl.lock.Lock()
	t, ok := tl.taskMap[taskKey(p, k)]
	if ok {
		// remove the task "lazily"
		// simply mark it as trash, so it'll be dropped when popped off the
		// queue.
		t.trash = true
	}
	tl.lock.Unlock()
}

type peerRequestTask struct {
	Entry  wantlist.Entry
	Target peer.ID // required

	// trash in a book-keeping field
	trash bool
	// created marks the time that the task was added to the queue
	created time.Time
	index   int // book-keeping field used by the pq container
}

// Key uniquely identifies a task.
func (t *peerRequestTask) Key() string {
	return taskKey(t.Target, t.Entry.Key)
}

func (t *peerRequestTask) Index() int {
	return t.index
}

func (t *peerRequestTask) SetIndex(i int) {
	t.index = i
}

// taskKey returns a key that uniquely identifies a task.
func taskKey(p peer.ID, k u.Key) string {
	return string(p.String() + k.String())
}

// FIFO is a basic task comparator that returns tasks in the order created.
var FIFO = func(a, b *peerRequestTask) bool {
	return a.created.Before(b.created)
}

// V1 respects the target peer's wantlist priority. For tasks involving
// different peers, the oldest task is prioritized.
var V1 = func(a, b *peerRequestTask) bool {
	if a.Target == b.Target {
		return a.Entry.Priority > b.Entry.Priority
	}
	return FIFO(a, b)
}

func wrapCmp(f func(a, b *peerRequestTask) bool) func(a, b pq.Elem) bool {
	return func(a, b pq.Elem) bool {
		return f(a.(*peerRequestTask), b.(*peerRequestTask))
	}
}
