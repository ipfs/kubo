package gc

import (
	"context"
	"errors"
	"fmt"
	"sort"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
)

var log = logging.Logger("gc")

// GC performs a mark and sweep garbage collection of the blocks in the blockstore
// first, it creates a 'marked' set and adds to it the following:
// - all recursively pinned blocks, plus all of their descendants (recursively)
// - bestEffortRoots, plus all of its descendants (recursively)
// - all directly pinned blocks
// - all blocks utilized internally by the pinner
//
// The routine then iterates over every block in the blockstore and
// deletes any block that is not found in the marked set.
//
type GC interface {
	AddPinSource(...pin.Source) error
	Run(ctx context.Context) <-chan Result
}

type gctype struct {
	bs bstore.GCBlockstore
	ls dag.LinkService

	roots []pin.Source
}

var _ GC = (*gctype)(nil)

// NewGC creates new instance of garbage collector
func NewGC(bs bstore.GCBlockstore, ls dag.LinkService) (GC, error) {
	return &gctype{
		bs: bs,
		ls: ls.GetOfflineLinkService(),
	}, nil
}

// AddPinSource adds as pin.Source to be considered by the GC.
// Any calls to AddPinSource have to be done before any calls to Run.
func (g *gctype) AddPinSource(s ...pin.Source) error {
	g.roots = append(g.roots, s...)
	sort.SliceStable(g.roots, func(i, j int) bool {
		return g.roots[i].SortValue() < g.roots[j].SortValue()
	})

	return nil
}

// Run starts the garbage collector and continues it async
func (g *gctype) Run(ctx context.Context) <-chan Result {
	output := make(chan Result, 128)
	tri := newTriset()

	emark := log.EventBegin(ctx, "GC.mark")

	for _, r := range g.roots {
		// Stop adding roots at first Direct pin
		if r.Direct {
			break
		}
		cids, err := r.Get()
		if err != nil {
			output <- Result{Error: err}
			return output
		}
		for _, c := range cids {
			tri.InsertGray(c, r.Strict)
		}
	}
	go g.gcAsync(ctx, emark, output, tri)

	return output
}

func (g *gctype) gcAsync(ctx context.Context, emark *logging.EventInProgress,
	output chan Result, tri *triset) {

	defer close(output)

	bestEffortGetLinks := func(ctx context.Context, cid *cid.Cid) ([]*node.Link, error) {
		links, err := g.ls.GetLinks(ctx, cid)
		if err != nil && err != dag.ErrNotFound {
			return nil, &CannotFetchLinksError{cid, err}
		}
		return links, nil
	}

	getLinks := func(ctx context.Context, cid *cid.Cid) ([]*node.Link, error) {
		links, err := g.ls.GetLinks(ctx, cid)
		if err != nil {
			return nil, &CannotFetchLinksError{cid, err}
		}
		return links, nil
	}

	var criticalError error
	defer func() {
		if criticalError != nil {
			output <- Result{Error: criticalError}
		}
	}()

	// Enumerate without the lock
	for {
		finished, err := tri.EnumerateStep(ctx, bestEffortGetLinks, getLinks)
		if err != nil {
			output <- Result{Error: err}
			criticalError = ErrCannotFetchAllLinks
		}
		if finished {
			break
		}
	}
	if criticalError != nil {
		return
	}

	// Add white objects
	keychan, err := g.bs.AllKeysChan(ctx)
	if err != nil {
		output <- Result{Error: err}
		return
	}

loop:
	for {
		select {
		case c, ok := <-keychan:
			if !ok {
				break loop
			}
			tri.InsertFresh(c)
		case <-ctx.Done():
			fmt.Printf("ctx done\n")
			output <- Result{Error: ctx.Err()}
			return
		}
	}

	// Take the lock
	unlocker, elock := getGCLock(ctx, g.bs)

	defer unlocker.Unlock()
	defer elock.Done()

	// Add the roots again, they might have changed
	for _, r := range g.roots {
		cids, err := r.Get()
		if err != nil {
			criticalError = err
			return
		}
		for _, c := range cids {
			if !r.Direct {
				tri.InsertGray(c, r.Strict)
			} else {
				// this special case prevents incremental and concurrent GC
				tri.blacken(c, enumStrict)
			}
		}
	}

	// Reenumerate, fast as most will be duplicate
	for {
		finished, err := tri.EnumerateStep(ctx, getLinks, bestEffortGetLinks)
		if err != nil {
			output <- Result{Error: err}
			criticalError = ErrCannotFetchAllLinks
		}
		if finished {
			break
		}
	}

	if criticalError != nil {
		return
	}

	emark.Done()
	esweep := log.EventBegin(ctx, "GC.sweep")

	var whiteSetSize, blackSetSize uint64

loop2:
	for v, e := range tri.colmap {
		if e.getColor() != tri.white {
			blackSetSize++
			continue
		}
		whiteSetSize++

		c, err := cid.Cast([]byte(v))
		if err != nil {
			// this should not happen
			panic("error in cast of cid: " + err.Error())
		}

		err = g.bs.DeleteBlock(c)
		if err != nil {
			output <- Result{Error: &CannotDeleteBlockError{c, err}}
			criticalError = ErrCannotDeleteSomeBlocks
			continue
		}
		select {
		case output <- Result{KeyRemoved: c}:
		case <-ctx.Done():
			break loop2
		}
	}

	esweep.Append(logging.LoggableMap{
		"whiteSetSize": fmt.Sprintf("%d", whiteSetSize),
		"blackSetSize": fmt.Sprintf("%d", blackSetSize),
	})
	esweep.Done()
}

// Result represents an incremental output from a garbage collection
// run.  It contains either an error, or the cid of a removed object.
type Result struct {
	KeyRemoved *cid.Cid
	Error      error
}

func getGCLock(ctx context.Context, bs bstore.GCBlockstore) (bstore.Unlocker, *logging.EventInProgress) {
	elock := log.EventBegin(ctx, "GC.lockWait")
	unlocker := bs.GCLock()
	elock.Done()
	elock = log.EventBegin(ctx, "GC.locked")
	return unlocker, elock
}

func addRoots(tri *triset, pn pin.Pinner, bestEffortRoots []*cid.Cid) {
	for _, v := range bestEffortRoots {
		tri.InsertGray(v, false)
	}

	for _, v := range pn.RecursiveKeys() {
		tri.InsertGray(v, true)
	}

}

var ErrCannotFetchAllLinks = errors.New("garbage collection aborted: could not retrieve some links")

var ErrCannotDeleteSomeBlocks = errors.New("garbage collection incomplete: could not delete some blocks")

type CannotFetchLinksError struct {
	Key *cid.Cid
	Err error
}

func (e *CannotFetchLinksError) Error() string {
	return fmt.Sprintf("could not retrieve links for %s: %s", e.Key, e.Err)
}

type CannotDeleteBlockError struct {
	Key *cid.Cid
	Err error
}

func (e *CannotDeleteBlockError) Error() string {
	return fmt.Sprintf("could not remove %s: %s", e.Key, e.Err)
}
