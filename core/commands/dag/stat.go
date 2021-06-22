package dagcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-merkledag/traverse"
	"github.com/ipfs/interface-go-ipfs-core/path"

	cmds "github.com/ipfs/go-ipfs-cmds"
	mdag "github.com/ipfs/go-merkledag"
)

func dagStat(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	progressive := req.Options[progressOptionName].(bool)

	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
	if err != nil {
		return err
	}

	if len(rp.Remainder()) > 0 {
		return fmt.Errorf("cannot return size for anything other than a DAG with a root CID")
	}

	nodeGetter := mdag.NewSession(req.Context, api.Dag())
	obj, err := nodeGetter.Get(req.Context, rp.Cid())
	if err != nil {
		return err
	}

	dagstats := &DagStat{}
	err = traverse.Traverse(obj, traverse.Options{
		DAG:   nodeGetter,
		Order: traverse.DFSPre,
		Func: func(current traverse.State) error {
			dagstats.Size += uint64(len(current.Node.RawData()))
			dagstats.NumBlocks++

			if progressive {
				if err := res.Emit(dagstats); err != nil {
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

	if !progressive {
		if err := res.Emit(dagstats); err != nil {
			return err
		}
	}

	return nil
}

func finishCLIStat(res cmds.Response, re cmds.ResponseEmitter) error {
	var dagStats *DagStat
	for {
		v, err := res.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		out, ok := v.(*DagStat)
		if !ok {
			return e.TypeErr(out, v)
		}
		dagStats = out
		fmt.Fprintf(os.Stderr, "%v\r", out)
	}
	return re.Emit(dagStats)
}
