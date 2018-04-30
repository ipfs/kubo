package hamt

import (
	"context"
	//"fmt"
	//"os"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"
)

var log = logging.Logger("hamt")

// fetcher implements a background fether to retrieve missing child
// shards in large batches.  It attempts to retrieves the missing
// shards in an order that allow streaming of the complete hamt
// directory, assuming a depth first traversal.
type fetcher struct {
	// note: the fields in this structure should only be accesses by
	// the 'mainLoop' go routine, all communication should be done via
	// channels

	ctx   context.Context
	dserv ipld.DAGService

	newJob chan *Shard
	reqRes chan *Shard
	result chan result

	idle bool

	done chan int

	todoFirst *job            // do this job first since we are waiting for its results
	todo      jobStack        // stack of jobs that still need to be done
	jobs      map[*Shard]*job // map of all jobs in which the results have not been collected yet

	// stats relevent for streaming the complete hamt directory
	doneCnt    int // job's done but results not yet retrieved
	hits       int // job's result already ready, no delay
	nearMisses int // job currently being worked on, small delay
	misses     int // job on todo stack but will be done in the next batch, larger delay

	// other useful stats
	cidCnt int
}

// batchSize must be at least as large as the largest number of cids
// requested in a single job.  For best perforamce it should likely be
// sligtly larger as jobs are poped from the todo stack in order and a
// job close to the batchSize could force a very small batch to run.
// Recommend minimum size: 256 + 64 = 320
const batchSize = 320

//
// fetcher public interface
//

// startFetcher starts a new fetcher in the background
func startFetcher(ctx context.Context, dserv ipld.DAGService) *fetcher {
	log.Infof("fetcher: starting...")
	f := &fetcher{
		ctx:    ctx,
		dserv:  dserv,
		newJob: make(chan *Shard, 16),
		reqRes: make(chan *Shard),
		result: make(chan result),
		idle:   true,
		done:   make(chan int),
		jobs:   make(map[*Shard]*job),
	}
	go f.mainLoop()
	return f
}

// result contains the result of a job, see getResult
type result struct {
	vals map[string]*Shard
	errs []error
}

// get recursively gets the missing child shards for the hamt object.
// The missing children for the passed in shard is returned.  The
// children of the children are recursively retrieved in the
// background.  The result is the result of the batch request and not
// just the single job.  In particular, if the 'errs' field is empty
// the 'vals' of the result is guaranteed to contain the all the
// missing child shards, but the map may also contain child shards of
// other jobs in the batch
func (f *fetcher) get(hamt *Shard) result {
	f.reqRes <- hamt
	res := <-f.result
	return res
}

//
// fetcher internals
//

type job struct {
	id  *Shard
	idx int /* index in the todo stack, an index of -1 means the job
	   is already done or being worked on now */
	cids []*cid.Cid
	res  result
}

type jobStack struct {
	c []*job
}

func (f *fetcher) mainLoop() {
	var want *Shard
	for {
		select {
		case j := <-f.newJob:
			f.mainLoopAddJob(j)
		case id := <-f.reqRes:
			if want != nil {
				// programmer error
				panic("fetcher: can not request more than one result at a time")
			}
			j, ok := f.jobs[id]
			if !ok {
				j = f.mainLoopAddJob(id)
				if j == nil {
					// no children that need to be retrieved
					f.result <- result{vals: make(map[string]*Shard)}
				}
			}
			if j.res.vals != nil {
				f.hits++
				delete(f.jobs, id)
				f.doneCnt--
				f.result <- j.res
			} else {
				if j.idx != -1 {
					f.misses++
					// move job to todoFirst so that it will be done on the
					// next batch job
					f.todo.remove(j)
					f.todoFirst = j
				} else {
					f.nearMisses++
				}
				want = id
			}
		case cnt := <-f.done:
			f.doneCnt += cnt
			f.launch()
			log.Infof("fetcher: batch job done")
			log.Infof("fetcher stats (done, hits, nearMisses, misses): %d %d %d %d", f.doneCnt, f.hits, f.nearMisses, f.misses)
			if want != nil {
				j := f.jobs[want]
				if j.res.vals != nil {
					delete(f.jobs, want)
					f.doneCnt--
					f.result <- j.res
					want = nil
				}
			}
		case <-f.ctx.Done():
			log.Infof("fetcher: exiting")
			log.Infof("fetcher stats (done, hits, nearMisses, misses): %d %d %d %d", f.doneCnt, f.hits, f.nearMisses, f.misses)
			log.Infof("fetcher total number of CIDs retrieved: %d", f.cidCnt)
			return
		}
	}
}

// addJob adds a job to retrive the missing child shards for the
// provided shard
func (f *fetcher) mainLoopAddJob(hamt *Shard) *job {
	children := hamt.missingChildShards()
	if len(children) == 0 {
		return nil
	}
	j := &job{id: hamt, cids: children}
	if len(j.cids) > batchSize {
		// programmer error
		panic("job size larger than batchSize")
	}
	f.cidCnt += len(j.cids)
	f.todo.push(j)
	f.jobs[j.id] = j
	if f.idle {
		f.launch()
	}
	return j
}

type batchJob struct {
	cids []*cid.Cid
	jobs []*job
}

func (b *batchJob) add(j *job) {
	b.cids = append(b.cids, j.cids...)
	b.jobs = append(b.jobs, j)
	j.idx = -1
}

func (f *fetcher) launch() {
	bj := batchJob{}

	// always do todoFirst
	if f.todoFirst != nil {
		bj.add(f.todoFirst)
		f.todoFirst = nil
	}

	// pop requets from todo list until we hit the batchSize
	for !f.todo.empty() && len(bj.cids)+len(f.todo.top().cids) <= batchSize {
		j := f.todo.pop()
		bj.add(j)
	}

	if len(bj.cids) == 0 {
		log.Infof("fetcher: entering idle state: no more jobs")
		f.idle = true
		return
	}

	// launch job
	log.Infof("fetcher: starting batch job, size = %d", len(bj.cids))
	f.idle = false
	go func() {
		ch := f.dserv.GetMany(f.ctx, bj.cids)
		fetched := result{vals: make(map[string]*Shard)}
		for no := range ch {
			if no.Err != nil {
				fetched.errs = append(fetched.errs, no.Err)
				continue
			}
			hamt, err := NewHamtFromDag(f.dserv, no.Node)
			if err != nil {
				fetched.errs = append(fetched.errs, err)
				continue
			}
			f.newJob <- hamt
			fetched.vals[string(no.Node.Cid().Bytes())] = hamt
		}
		for _, job := range bj.jobs {
			job.res = fetched
		}
		f.done <- len(bj.jobs)
	}()
}

func (js *jobStack) empty() bool {
	return len(js.c) == 0
}

func (js *jobStack) top() *job {
	return js.c[len(js.c)-1]
}

func (js *jobStack) push(j *job) {
	j.idx = len(js.c)
	js.c = append(js.c, j)
}

func (js *jobStack) pop() *job {
	j := js.top()
	js.remove(j)
	return j
}

func (js *jobStack) remove(j *job) {
	js.c[j.idx] = nil
	j.idx = -1
	js.popEmpty()
}

func (js *jobStack) popEmpty() {
	for len(js.c) > 0 && js.c[len(js.c)-1] == nil {
		js.c = js.c[:len(js.c)-1]
	}
}
