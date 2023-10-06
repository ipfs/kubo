package dagcmd

import (
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func dagResolve(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
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

	return cmds.EmitOnce(res, &ResolveOutput{
		Cid:     rp.RootCid(),
		RemPath: path.SegmentsToString(remainder...),
	})
}
