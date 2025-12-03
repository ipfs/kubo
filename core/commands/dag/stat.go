package dagcmd

import (
	"fmt"
	"io"
	"os"

	mdag "github.com/ipfs/boxo/ipld/merkledag"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/ipfs/kubo/core/commands/e"
)

// cidStatCache caches the statistics for already-traversed CIDs to avoid
// redundant traversals when multiple DAGs share common subgraphs
type cidStatCache struct {
	stats map[string]*cachedStat
}

type cachedStat struct {
	size      uint64
	numBlocks int64
}

func newCidStatCache() *cidStatCache {
	return &cidStatCache{
		stats: make(map[string]*cachedStat),
	}
}

func (c *cidStatCache) get(cid cid.Cid) (*cachedStat, bool) {
	stat, ok := c.stats[cid.String()]
	return stat, ok
}

func (c *cidStatCache) put(cid cid.Cid, size uint64, numBlocks int64) {
	c.stats[cid.String()] = &cachedStat{
		size:      size,
		numBlocks: numBlocks,
	}
}

func dagStat(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	progressive := req.Options[progressOptionName].(bool)
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}
	nodeGetter := mdag.NewSession(req.Context, api.Dag())

	cidSet := cid.NewSet()
	cache := newCidStatCache()
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

		dagstats := &DagStat{Cid: rp.RootCid()}
		dagStatSummary.appendStats(dagstats)

		// Use a custom recursive traversal with DP caching
		var traverseWithCache func(c cid.Cid) (*cachedStat, error)
		traverseWithCache = func(c cid.Cid) (*cachedStat, error) {
			// Check cache first - this is the DP optimization
			// If cached, just return the stats without updating global counters
			if cached, ok := cache.get(c); ok {
				// Still need to track redundant access
				node, err := nodeGetter.Get(req.Context, c)
				if err != nil {
					return nil, err
				}
				nodeSize := uint64(len(node.RawData()))
				dagStatSummary.incrementRedundantSize(nodeSize)
				cidSet.Add(c)

				if progressive {
					if err := res.Emit(dagStatSummary); err != nil {
						return nil, err
					}
				}
				return cached, nil
			}

			node, err := nodeGetter.Get(req.Context, c)
			if err != nil {
				return nil, err
			}

			nodeSize := uint64(len(node.RawData()))
			subtreeSize := nodeSize
			subtreeBlocks := int64(1)

			// Update global tracking for this new node
			if !cidSet.Has(c) {
				dagStatSummary.incrementTotalSize(nodeSize)
			}
			dagStatSummary.incrementRedundantSize(nodeSize)
			cidSet.Add(c)

			// Recursively compute stats for all children
			for _, link := range node.Links() {
				childStats, err := traverseWithCache(link.Cid)
				if err != nil {
					return nil, err
				}
				subtreeSize += childStats.size
				subtreeBlocks += childStats.numBlocks
			}

			// Cache this node's subtree stats
			stat := &cachedStat{
				size:      subtreeSize,
				numBlocks: subtreeBlocks,
			}
			cache.put(c, subtreeSize, subtreeBlocks)

			if progressive {
				if err := res.Emit(dagStatSummary); err != nil {
					return nil, err
				}
			}

			return stat, nil
		}

		rootStats, err := traverseWithCache(rp.RootCid())
		if err != nil {
			return fmt.Errorf("error traversing DAG: %w", err)
		}

		dagstats.Size = rootStats.size
		dagstats.NumBlocks = rootStats.numBlocks
	}

	dagStatSummary.UniqueBlocks = cidSet.Len()
	dagStatSummary.calculateSummary()

	if err := res.Emit(dagStatSummary); err != nil {
		return err
	}
	return nil
}

func finishCLIStat(res cmds.Response, re cmds.ResponseEmitter) error {
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
			if dagStats.Ratio == 0 {
				length := len(dagStats.DagStatsArray)
				if length > 0 {
					currentStat := dagStats.DagStatsArray[length-1]
					fmt.Fprintf(os.Stderr, "CID: %s, Size: %d, NumBlocks: %d\n", currentStat.Cid, currentStat.Size, currentStat.NumBlocks)
				}
			}
		default:
			return e.TypeErr(out, v)

		}
	}
	return re.Emit(dagStats)
}
