package dagcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/dustin/go-humanize"
	mdag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/merkledag/traverse"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/ipfs/kubo/core/commands/e"
)

// TODO cache every cid traversal in a dp cache
// if the cid exists in the cache, don't traverse it, and use the cached result
// to compute the new state

func dagStat(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	// Default to true (emit intermediate states) for HTTP/RPC clients that want progress
	progressive := true
	if val, specified := req.Options[progressOptionName].(bool); specified {
		progressive = val
	}
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	enc, err := cmdenv.GetCidEncoder(req)
	if err != nil {
		return err
	}

	nodeGetter := mdag.NewSession(req.Context, api.Dag())

	// boxo's Traverse (SkipDuplicates below) keeps its own per-root "seen" set,
	// so within a single root it never visits the same CID twice. A command-level
	// cidSet is needed only to dedup *across* multiple roots, for the global
	// UniqueBlocks and TotalSize counts. With one root it would hold a second
	// copy of every CID the traversal already tracks, doubling the memory spent
	// just on remembering which CIDs were seen. On very large DAGs that second
	// copy has OOM-killed daemons running `dag stat` over multi-hundred-GiB
	// UnixFS roots, so allocate it only when there is more than one root.
	var cidSet *cid.Set
	if len(req.Arguments) > 1 {
		cidSet = cid.NewSet()
	}
	dagStatSummary := &DagStatSummary{DagStatsArray: []*DagStat{}}
	for _, a := range req.Arguments {
		p, err := cmdutils.PathOrCidPath(a)
		if err != nil {
			return err
		}
		rp, remainder, err := api.ResolvePath(req.Context, p)
		if err != nil {
			return err
		}
		if len(remainder) > 0 {
			return fmt.Errorf("cannot return size for anything other than a DAG with a root CID")
		}

		obj, err := nodeGetter.Get(req.Context, rp.RootCid())
		if err != nil {
			return err
		}
		dagstats := &DagStat{Cid: enc.Encode(rp.RootCid())}
		dagStatSummary.appendStats(dagstats)
		err = traverse.Traverse(obj, traverse.Options{
			DAG:   nodeGetter,
			Order: traverse.DFSPre,
			Func: func(current traverse.State) error {
				currentNodeSize := uint64(len(current.Node.RawData()))
				dagstats.Size += currentNodeSize
				dagstats.NumBlocks++
				// With a single root, boxo's SkipDuplicates guarantees each CID is
				// visited exactly once, so every block counts toward the global
				// unique total. With multiple roots, cidSet tracks which CIDs were
				// already counted under a previous root (Visit reports true the
				// first time a CID is seen).
				if cidSet == nil || cidSet.Visit(current.Node.Cid()) {
					dagStatSummary.incrementTotalSize(currentNodeSize)
				}
				dagStatSummary.incrementRedundantSize(currentNodeSize)
				if progressive {
					if err := res.Emit(dagStatSummary); err != nil {
						return err
					}
				}
				return nil
			},
			ErrFunc:        nil,
			SkipDuplicates: true,
		})
		if err != nil {
			return fmt.Errorf("error traversing DAG: %w", err)
		}
	}

	if cidSet != nil {
		dagStatSummary.UniqueBlocks = cidSet.Len()
	} else {
		// Single root: boxo deduplicated within the traversal, so the number of
		// unique blocks equals the number of blocks visited.
		for _, ds := range dagStatSummary.DagStatsArray {
			dagStatSummary.UniqueBlocks += int(ds.NumBlocks)
		}
	}
	dagStatSummary.calculateSummary()

	if err := res.Emit(dagStatSummary); err != nil {
		return err
	}
	return nil
}

func finishCLIStat(res cmds.Response, re cmds.ResponseEmitter) error {
	showProgress := cmdenv.ShouldShowProgress(res.Request(), progressOptionName)

	var dagStats *DagStatSummary
	for {
		v, err := res.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch out := v.(type) {
		case *DagStatSummary:
			dagStats = out
			// Ratio == 0 means this is a progress update (not final result)
			if showProgress && dagStats.Ratio == 0 {
				// Sum up total progress across all DAGs being scanned
				var totalBlocks int64
				var totalSize uint64
				for _, stat := range dagStats.DagStatsArray {
					totalBlocks += stat.NumBlocks
					totalSize += stat.Size
				}
				fmt.Fprintf(os.Stderr, "Fetched/Processed %d blocks, %d bytes (%s)\r", totalBlocks, totalSize, humanize.Bytes(totalSize))
			}
		default:
			return e.TypeErr(out, v)
		}
	}

	// Clear the progress line before final output
	if showProgress {
		fmt.Fprint(os.Stderr, "\033[2K\r")
	}

	return re.Emit(dagStats)
}
