package dagcmd

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/ipfs/boxo/path"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	"github.com/ipld/go-ipld-prime/traversal"
	mc "github.com/multiformats/go-multicodec"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func dagGet(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	codecStr, _ := req.Options["output-codec"].(string)
	var codec mc.Code
	if err := codec.Set(codecStr); err != nil {
		return err
	}

	p, err := cmdutils.PathOrCidPath(req.Arguments[0])
	if err != nil {
		return err
	}

	rp, remainder, err := api.ResolvePath(req.Context, p)
	if err != nil {
		return err
	}

	obj, err := api.Dag().Get(req.Context, rp.RootCid())
	if err != nil {
		return err
	}

	universal, ok := obj.(ipldlegacy.UniversalNode)
	if !ok {
		return fmt.Errorf("%T is not a valid IPLD node", obj)
	}

	finalNode := universal.(ipld.Node)

	if len(remainder) > 0 {
		remainderPath := ipld.ParsePath(path.SegmentsToString(remainder...))

		finalNode, err = traversal.Get(finalNode, remainderPath)
		if err != nil {
			return err
		}
	}

	encoder, err := multicodec.LookupEncoder(uint64(codec))
	if err != nil {
		return fmt.Errorf("invalid encoding: %s - %s", codec, err)
	}

	r, w := io.Pipe()
	go func() {
		defer w.Close()
		// Encoding runs the codec named by the node's CID, so it runs
		// third-party code. This goroutine is detached from the request, and a
		// panic on it would end the daemon rather than the command.
		defer func() {
			if rec := recover(); rec != nil {
				log.Errorf("recovered from panic encoding %s as %s: %v\n%s", p, codec, rec, debug.Stack())
				_ = w.CloseWithError(errors.New("internal error encoding node"))
			}
		}()
		if err := encoder(finalNode, w); err != nil {
			_ = w.CloseWithError(err)
		}
	}()

	return res.Emit(r)
}
