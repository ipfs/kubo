package commands

import (
	"sync"
	"time"
)

// ReqLogEntry is an entry in the request log.
type ReqLogEntry struct {
	StartTime time.Time
	EndTime   time.Time
	Active    bool
	Command   string
	Options   map[string]interface{}
	Args      []string
	ID        int

	log *ReqLog
}

// Copy returns a copy of the ReqLogEntry.
func (r *ReqLogEntry) Copy() *ReqLogEntry {
	out := *r
	out.log = nil
	return &out
}

// ReqLog is a log of requests.
type ReqLog struct {
	Requests []*ReqLogEntry
	nextID   int
	lock     sync.Mutex
	keep     time.Duration
}

// AddEntry adds an entry to the log.
func (rl *ReqLog) AddEntry(rle *ReqLogEntry) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rle.ID = rl.nextID
	rl.nextID++
	rl.Requests = append(rl.Requests, rle)

	if !rle.Active {
		rl.maybeCleanup()
	}
}

// ClearInactive removes stale entries.
func (rl *ReqLog) ClearInactive() {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	k := rl.keep
	rl.keep = 0
	rl.cleanup()
	rl.keep = k
}

func (rl *ReqLog) maybeCleanup() {
	// only do it every so often or it might
	// become a perf issue
	if len(rl.Requests)%10 == 0 {
		rl.cleanup()
	}
}

func (rl *ReqLog) cleanup() {
	i := 0
	now := time.Now()
	for j := 0; j < len(rl.Requests); j++ {
		rj := rl.Requests[j]
		if rj.Active || rl.Requests[j].EndTime.Add(rl.keep).After(now) {
			rl.Requests[i] = rl.Requests[j]
			i++
		}
	}
	rl.Requests = rl.Requests[:i]
}

// SetKeepTime sets a duration after which an entry will be considered inactive.
func (rl *ReqLog) SetKeepTime(t time.Duration) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	rl.keep = t
}

// Report generates a copy of all the entries in the requestlog.
func (rl *ReqLog) Report() []*ReqLogEntry {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	out := make([]*ReqLogEntry, len(rl.Requests))

	for i, e := range rl.Requests {
		out[i] = e.Copy()
	}

	return out
}

// Finish marks an entry in the log as finished.
func (rl *ReqLog) Finish(rle *ReqLogEntry) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rle.Active = false
	rle.EndTime = time.Now()

	rl.maybeCleanup()
}
