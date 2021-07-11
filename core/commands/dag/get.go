package dagcmd

import (
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/interface-go-ipfs-core/path"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func dagGet(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
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

	var out interface{} = obj
	if len(rp.Remainder()) > 0 {
		rem := strings.Split(rp.Remainder(), "/")
		final, _, err := obj.Resolve(rem)
		if err != nil {
			return err
		}
		out = final
	}
	return cmds.EmitOnce(res, &out)
}
