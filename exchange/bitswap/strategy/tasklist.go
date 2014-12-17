package strategy

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO: at some point, the strategy needs to plug in here
// to help decide how to sort tasks (on add) and how to select
// tasks (on getnext). For now, we are assuming a dumb/nice strategy.
type taskList struct {
	tasks   []*Task
	taskmap map[string]*Task
}

func newTaskList() *taskList {
	return &taskList{
		taskmap: make(map[string]*Task),
	}
}

type Task struct {
	Key           u.Key
	Target        peer.Peer
	theirPriority int
}

// Push currently adds a new task to the end of the list
// TODO: make this into a priority queue
func (tl *taskList) Push(block u.Key, priority int, to peer.Peer) {
	if task, ok := tl.taskmap[taskKey(to, block)]; ok {
		// TODO: when priority queue is implemented,
		//       rearrange this Task
		task.theirPriority = priority
		return
	}
	task := &Task{
		Key:           block,
		Target:        to,
		theirPriority: priority,
	}
	tl.tasks = append(tl.tasks, task)
	tl.taskmap[taskKey(to, block)] = task
}

// Pop 'pops' the next task to be performed. Returns nil no task exists.
func (tl *taskList) Pop() *Task {
	var out *Task
	for len(tl.tasks) > 0 {
		// TODO: instead of zero, use exponential distribution
		//       it will help reduce the chance of receiving
		//		 the same block from multiple peers
		out = tl.tasks[0]
		tl.tasks = tl.tasks[1:]
		delete(tl.taskmap, taskKey(out.Target, out.Key))
		// Filter out blocks that have been cancelled
		if out.theirPriority >= 0 {
			break
		}
	}

	return out
}

// Cancel lazily cancels the sending of a block to a given peer
func (tl *taskList) Cancel(k u.Key, p peer.Peer) {
	t, ok := tl.taskmap[taskKey(p, k)]
	if ok {
		t.theirPriority = -1
	}
}

// taskKey returns a key that uniquely identifies a task.
func taskKey(p peer.Peer, k u.Key) string {
	return string(p.Key() + k)
}
