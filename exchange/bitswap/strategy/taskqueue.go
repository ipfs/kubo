package strategy

import (
	wantlist "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO: at some point, the strategy needs to plug in here
// to help decide how to sort tasks (on add) and how to select
// tasks (on getnext). For now, we are assuming a dumb/nice strategy.
type taskQueue struct {
	tasks   []*task
	taskmap map[string]*task
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		taskmap: make(map[string]*task),
	}
}

type task struct {
	Entry  wantlist.Entry
	Target peer.Peer
	Trash  bool
}

// Push currently adds a new task to the end of the list
// TODO: make this into a priority queue
func (tl *taskQueue) Push(block u.Key, priority int, to peer.Peer) {
	if task, ok := tl.taskmap[taskKey(to, block)]; ok {
		// TODO: when priority queue is implemented,
		//       rearrange this task
		task.Entry.Priority = priority
		return
	}
	task := &task{
		Entry: wantlist.Entry{
			Key:      block,
			Priority: priority,
		},
		Target: to,
	}
	tl.tasks = append(tl.tasks, task)
	tl.taskmap[taskKey(to, block)] = task
}

// Pop 'pops' the next task to be performed. Returns nil no task exists.
func (tl *taskQueue) Pop() *task {
	var out *task
	for len(tl.tasks) > 0 {
		// TODO: instead of zero, use exponential distribution
		//       it will help reduce the chance of receiving
		//		 the same block from multiple peers
		out = tl.tasks[0]
		tl.tasks = tl.tasks[1:]
		delete(tl.taskmap, taskKey(out.Target, out.Entry.Key))
		if out.Trash {
			continue // discarding tasks that have been removed
		}
		break // and return |out|
	}
	return out
}

// Remove lazily removes a task from the queue
func (tl *taskQueue) Remove(k u.Key, p peer.Peer) {
	t, ok := tl.taskmap[taskKey(p, k)]
	if ok {
		t.Trash = true
	}
}

// taskKey returns a key that uniquely identifies a task.
func taskKey(p peer.Peer, k u.Key) string {
	return string(p.Key() + k)
}
