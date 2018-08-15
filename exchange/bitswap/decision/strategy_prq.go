// Strategy PRQ
// ============

package decision

import (
	"sync"
	"time"

	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	pq "gx/ipfs/QmZUbTDJ39JpvtFCSubiWeUTQRvMA1tVE5RZCJrY4oeAsC/go-ipfs-pq"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// Type and Constructor
// --------------------

func newSPRQ(rrqCfg *RRQConfig) *sprq {
	return &sprq{
		taskMap:  make(map[string]*peerRequestTask),
		partners: make(map[peer.ID]*activePartner),
		pQueue:   pq.New(partnerCompare),
		rrq:      newRRQueue(rrqCfg),
	}
}

// verify interface implementation
var _ peerRequestQueue = &sprq{}

type sprq struct {
	lock     sync.Mutex
	pQueue   pq.PQ
	taskMap  map[string]*peerRequestTask
	partners map[peer.ID]*activePartner
	rrq      *RRQueue
}

// Push
// ----

// Push adds a new peerRequestTask to the end of the list
func (tl *sprq) Push(entry *wantlist.Entry, receipt *Receipt) {
	to := peer.ID(receipt.Peer)
	tl.lock.Lock()
	defer tl.lock.Unlock()

	partner, ok := tl.partners[to]

	if !ok {
		partner = newActivePartner()
		tl.pQueue.Push(partner)
		tl.partners[to] = partner
	}

	partner.activelk.Lock()
	defer partner.activelk.Unlock()

	if partner.activeBlocks.Has(entry.Cid) {
		return
	}

	if task, ok := tl.taskMap[taskKey(to, entry.Cid)]; ok {
		task.Entry.Priority = entry.Priority
		partner.taskQueue.Update(task.index)
		return
	}

	task := &peerRequestTask{
		Entry:   entry,
		Target:  to,
		created: time.Now(),
		Done: func() {
			tl.lock.Lock()
			partner.TaskDone(entry.Cid)
			tl.pQueue.Update(partner.Index())
			tl.lock.Unlock()
		},
	}

	partner.taskQueue.Push(task)
	tl.taskMap[task.Key()] = task
	partner.requests++
	tl.pQueue.Update(partner.Index())

	tl.rrq.UpdateWeight(to, receipt)
}

// Pop
// ---

// Pop 'pops' the next task to be performed. Returns nil if no task exists.
func (tl *sprq) Pop() *peerRequestTask {
	tl.lock.Lock()
	defer tl.lock.Unlock()

	// get the next peer/task to serve
	rrp, task := tl.nextTask()
	if task == nil {
		return nil
	}
	partner := tl.partners[rrp.id]

	// start the task
	partner.StartTask(task.Entry.Cid)
	partner.requests--

	rrp.allocation -= task.Entry.Size

	if rrp.allocation == 0 {
		// peer has reached allocation limit for this round, remove peer from queue
		tl.rrq.Pop()
	}
	return task
}

// nextTask() uses the `RRQueue` and peer `taskQueue`s to determine the next
// request to serve
func (tl *sprq) nextTask() (rrp *RRPeer, task *peerRequestTask) {
	if tl.pQueue.Len() == 0 {
		return nil, nil
	}

	if tl.rrq.NumPeers() == 0 {
		// may have finished last RR round, reallocate requests to peers
		tl.rrq.InitRound()
		if tl.rrq.NumPeers() == 0 {
			// if allocations still empty, there are no requests to serve
			return nil, nil
		}
	}

	// figure out which peer should be served next
	for tl.rrq.NumPeers() > 0 {
		rrp = tl.rrq.Head()

		task := tl.partnerNextTask(tl.partners[rrp.id])
		// nil task means this peer has no valid tasks to be served at the moment
		if task == nil {
			tl.rrq.Pop()
			continue
		}

		// check whether |task| exceeds peer's round-robin allocation
		if task.Entry.Size > rrp.allocation {
			tl.partners[rrp.id].taskQueue.Push(task)
			tl.rrq.Pop()
			continue
		}

		return rrp, task
	}
	return nil, nil
}

// get first non-trash task
func (tl *sprq) partnerNextTask(partner *activePartner) *peerRequestTask {
	for partner.taskQueue.Len() > 0 {
		task := partner.taskQueue.Pop().(*peerRequestTask)
		// return task if it's not trash
		if !task.trash {
			return task
		}
	}
	return nil
}

// Remove
// ------

// Remove removes a task from the queue
func (tl *sprq) Remove(k *cid.Cid, p peer.ID) {
	tl.lock.Lock()
	t, ok := tl.taskMap[taskKey(p, k)]
	if ok {
		// remove the task "lazily"
		// simply mark it as trash, so it'll be dropped when popped off the
		// queue.
		t.trash = true

		// having canceled a block, we now account for that in the given partner
		partner := tl.partners[p]
		partner.requests--

		// we now also 'freeze' that partner. If they sent us a cancel for a
		// block we were about to send them, we should wait a short period of time
		// to make sure we receive any other in-flight cancels before sending
		// them a block they already potentially have
		//
		// TODO: figure out how to implement this for RRQ, e.g. move partner to end of
		// RRQ so that they still get their allocations but go at the end of the round
		// (but then also have to decide what to do if partner only peer in queue
		/*if partner.freezeVal == 0 {
			tl.frozen[p] = partner
		}*/

		//partner.freezeVal++
		tl.pQueue.Update(partner.index)
	}
	tl.lock.Unlock()
}

// Unimplemented
// -------------

func (tl *sprq) thawRound() {
}

// Helpers
// -------

func (tl *sprq) allocationForPeer(id peer.ID) int {
	for _, rrp := range tl.rrq.allocations {
		if rrp.id == id {
			return rrp.allocation
		}
	}
	return 0
}
