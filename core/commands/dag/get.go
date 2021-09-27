package dagcmd

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	"github.com/ipfs/interface-go-ipfs-core/path"

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

	rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
	if err != nil {
		return err
	}

	obj, err := api.Dag().Get(req.Context, rp.Cid())
	if err != nil {
		return err
	}

	universal, ok := obj.(ipldlegacy.UniversalNode)
	if !ok {
		return fmt.Errorf("%T is not a valid IPLD node", obj)
	}

	finalNode := universal.(ipld.Node)

	if len(rp.Remainder()) > 0 {
		remainderPath := ipld.ParsePath(rp.Remainder())

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
		if err := encoder(finalNode, w); err != nil {
			_ = w.CloseWithError(err)
		}
	}()

	return res.Emit(r)
}
