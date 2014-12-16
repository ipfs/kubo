package strategy

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO: at some point, the strategy needs to plug in here
// to help decide how to sort tasks (on add) and how to select
// tasks (on getnext). For now, we are assuming a dumb/nice strategy.
type TaskList struct {
	tasks   []*Task
	taskmap map[u.Key]*Task
}

func NewTaskList() *TaskList {
	return &TaskList{
		taskmap: make(map[u.Key]*Task),
	}
}

type Task struct {
	Key           u.Key
	Target        peer.Peer
	theirPriority int
}

// Push currently adds a new task to the end of the list
// TODO: make this into a priority queue
func (tl *TaskList) Push(block u.Key, priority int, to peer.Peer) {
	if task, ok := tl.taskmap[to.Key()+block]; ok {
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
	tl.taskmap[to.Key()+block] = task
}

// Pop returns the next task to be performed by bitswap the task is then
// removed from the list
func (tl *TaskList) Pop() *Task {
	var out *Task
	for len(tl.tasks) > 0 {
		// TODO: instead of zero, use exponential distribution
		//       it will help reduce the chance of receiving
		//		 the same block from multiple peers
		out = tl.tasks[0]
		tl.tasks = tl.tasks[1:]
		delete(tl.taskmap, out.Target.Key()+out.Key)
		// Filter out blocks that have been cancelled
		if out.theirPriority >= 0 {
			break
		}
	}

	return out
}

// Cancel lazily cancels the sending of a block to a given peer
func (tl *TaskList) Cancel(k u.Key, p peer.Peer) {
	t, ok := tl.taskmap[p.Key()+k]
	if ok {
		t.theirPriority = -1
	}
}
