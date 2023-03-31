package dagcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/ipfs/boxo/coreiface/path"
	mdag "github.com/ipfs/boxo/ipld/merkledag"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-merkledag/traverse"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/e"
)

func dagStat(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}
	nodeGetter := mdag.NewSession(req.Context, api.Dag())

	cidSet := cid.NewSet()
	dagStatCalculator := &DagStatCalculator{Summary: &DagStatSummary{}}
	for _, a := range req.Arguments {
		rp, err := api.ResolvePath(req.Context, path.New(a))
		if err != nil {
			return err
		}
		if len(rp.Remainder()) > 0 {
			return fmt.Errorf("cannot return size for anything other than a DAG with a root CID")
		}

		obj, err := nodeGetter.Get(req.Context, rp.Cid())
		if err != nil {
			return err
		}
		dagstats := &DagStat{Cid: rp.Cid()}
		err = traverse.Traverse(obj, traverse.Options{
			DAG:   nodeGetter,
			Order: traverse.DFSPre,
			Func: func(current traverse.State) error {
				dagstats.Size += uint64(len(current.Node.RawData()))
				dagstats.NumBlocks++
				if !cidSet.Has(current.Node.Cid()) {
					dagStatCalculator.Summary.incrementTotalSize(dagstats.Size)
				}
				dagStatCalculator.Summary.incrementRedundantSize(dagstats.Size)
				cidSet.Add(current.Node.Cid())
				return nil
			},
			ErrFunc:        nil,
			SkipDuplicates: true,
		})
		if err != nil {
			return fmt.Errorf("error traversing DAG: %w", err)
		}
		dagStatCalculator.CurrentDag = dagstats
		if err := res.Emit(dagStatCalculator); err != nil {
			return err
		}
	}

	dagStatCalculator.Summary.UniqueBlocks = cidSet.Len()
	dagStatCalculator.calculateSharedSize()
	dagStatCalculator.calculateRatio()
	dagStatCalculator.CurrentDag = nil
	if err := res.Emit(dagStatCalculator); err != nil {
		return err
	}
	return nil
}

func finishCLIStat(res cmds.Response, re cmds.ResponseEmitter) error {

	var dagStats *DagStatCalculator
	for {
		v, err := res.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch out := v.(type) {
		case *DagStatCalculator:
			dagStats = out
			if out.CurrentDag != nil {
				fmt.Fprint(os.Stderr, *out)
				fmt.Fprint(os.Stderr, out)
			}
		default:
			return e.TypeErr(out, v)
		}
	}
	return re.Emit(dagStats)
}
