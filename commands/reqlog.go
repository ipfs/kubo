package commands

import (
	"strings"
	"sync"
	"time"
)

type ReqLogEntry struct {
	StartTime time.Time
	EndTime   time.Time
	Active    bool
	Command   string
	Options   map[string]interface{}
	Args      []string
	ID        int

	req Request
	log *ReqLog
}

func (r *ReqLogEntry) Finish() {
	r.log.lock.Lock()
	defer r.log.lock.Unlock()

	r.Active = false
	r.EndTime = time.Now()

	r.log.maybeCleanup()
}

func (r *ReqLogEntry) Copy() *ReqLogEntry {
	out := *r
	out.log = nil
	return &out
}

type ReqLog struct {
	Requests []*ReqLogEntry
	nextID   int
	lock     sync.Mutex
}

func (rl *ReqLog) Add(req Request) *ReqLogEntry {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rle := &ReqLogEntry{
		StartTime: time.Now(),
		Active:    true,
		Command:   strings.Join(req.Path(), "/"),
		Options:   req.Options(),
		Args:      req.Arguments(),
		ID:        rl.nextID,
		req:       req,
		log:       rl,
	}

	rl.nextID++
	rl.Requests = append(rl.Requests, rle)
	return rle
}

func (rl *ReqLog) maybeCleanup() {
	// only do it every so often or it might
	// become a perf issue
	if len(rl.Requests) == 0 {
		rl.cleanup()
	}
}

func (rl *ReqLog) cleanup() {
	var i int
	for ; i < len(rl.Requests); i++ {
		req := rl.Requests[i]
		if req.Active || req.EndTime.Add(time.Hour).After(time.Now()) {
			break
		}
	}

	if i > 0 {
		var j int
		for i < len(rl.Requests) {
			rl.Requests[j] = rl.Requests[i]
			j++
			i++
		}
		rl.Requests = rl.Requests[:len(rl.Requests)-i]
	}
}

func (rl *ReqLog) Report() []*ReqLogEntry {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	out := make([]*ReqLogEntry, len(rl.Requests))

	for i, e := range rl.Requests {
		out[i] = e.Copy()
	}

	return out
}
