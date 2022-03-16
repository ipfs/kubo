package dagcmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/interface-go-ipfs-core/path"

	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	mdag "github.com/ipfs/go-merkledag"
	mh "github.com/multiformats/go-multihash"
)

// Implement traversal here because there is a lot of adhoc logic to take care off
// This is an async (multithreaded downloads but with a single contention point for processing) DFS implementation
type statTraversal struct {
	runners int
	stats   DagStat

	ctx           context.Context
	cancel        context.CancelFunc
	res           cmds.ResponseEmitter
	getter        ipld.NodeGetter
	nodes         chan *ipld.NodeOption
	progressive   bool
	skipRawleaves bool
	stated        map[string]struct{}
	seen          map[string]struct{}
}

func newStatTraversal(ctx context.Context, getter ipld.NodeGetter, res cmds.ResponseEmitter, progressive bool, skipRawleaves bool) *statTraversal {
	ctx, cancel := context.WithCancel(ctx)
	return &statTraversal{
		ctx:           ctx,
		cancel:        cancel,
		getter:        getter,
		res:           res,
		nodes:         make(chan *ipld.NodeOption),
		progressive:   progressive,
		skipRawleaves: skipRawleaves,
		// Use two different maps to correctly coiunt blocks with matching multihashes but different codecs
		stated: make(map[string]struct{}),
		seen:   make(map[string]struct{}),
	}
}

func (t *statTraversal) pump() error {
	defer t.cancel()
	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case n := <-t.nodes:
			if n.Err != nil {
				return n.Err
			}
			t.runners--
			err := t.handleStating(n.Node.Cid(), uint64(len(n.Node.RawData())))
			if err != nil {
				return err
			}
			err = t.handleRecursion(n.Node)
			if err != nil {
				return err
			}

			finished := t.runners == 0
			if t.progressive || finished {
				if err := t.res.Emit(&t.stats); err != nil {
					return err
				}
			}

			if finished {
				return nil
			}
		}
	}
}

func (t *statTraversal) handleStating(c cid.Cid, nodeLen uint64) error {
	k := string(c.Hash())
	if _, alreadyCounted := t.stated[k]; alreadyCounted {
		return nil
	}
	t.stated[k] = struct{}{}

	if c.Prefix().MhType != mh.IDENTITY { // Do not count the size of inlined blocks
		t.stats.Size += nodeLen
	}
	t.stats.NumBlocks++
	return nil
}

func (t *statTraversal) handleRecursion(node ipld.Node) error {
	scan := make([]cid.Cid, 0, len(node.Links())) // Prealoc enough capacity
	for _, l := range node.Links() {
		k := l.Cid.KeyString()
		if _, alreadySeen := t.seen[k]; alreadySeen {
			continue
		}
		t.seen[k] = struct{}{}

		if t.skipRawleaves {
			prefix := l.Cid.Prefix()
			if prefix.Codec == cid.Raw && l.Size != 0 /* still fetch links with likely missing size */ {
				err := t.handleStating(l.Cid, l.Size)
				if err != nil {
					return err
				}
				continue
			}
		}

		scan = append(scan, l.Cid)
	}

	t.runners += len(scan)
	go func() {
		c := t.getter.GetMany(t.ctx, scan)
		for {
			select {
			case <-t.ctx.Done():
				return
			case v, ok := <-c:
				if !ok {
					return
				}
				select {
				case <-t.ctx.Done():
					return
				case t.nodes <- v:
				}
			}
		}
	}()
	return nil
}

func (t *statTraversal) traverse(c cid.Cid) error {
	t.seen[c.KeyString()] = struct{}{}
	t.runners = 1
	go func() {
		node, err := t.getter.Get(t.ctx, c)
		select {
		case <-t.ctx.Done():
		case t.nodes <- &ipld.NodeOption{Node: node, Err: err}:
		}
	}()
	return t.pump()
}

func dagStat(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	progressive := req.Options[progressOptionName].(bool)
	skipRawleaves := req.Options[skipRawleavesOptionName].(bool)

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
	t := newStatTraversal(req.Context, nodeGetter, res, progressive, skipRawleaves)

	err = t.traverse(rp.Cid())
	if err != nil {
		return fmt.Errorf("error traversing DAG: %w", err)
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
