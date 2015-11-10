package commands

import (
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corerepo"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

const progressBarMinSize = 1024 * 1024 * 8 // show progress bar for outputs > 8MiB

var CatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show IPFS object data",
		ShortDescription: `
Retrieves the object named by <ipfs-or-ipns-path> and outputs the data
it contains.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !node.OnlineMode() {
			if err := node.SetupOfflineRouting(); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		readers, length, err := cat(req.Context(), node, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if err := corerepo.ConditionalGC(req.Context(), node, length); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetLength(length)

		reader := io.MultiReader(readers...)
		res.SetOutput(reader)
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Length() < progressBarMinSize {
			return
		}

		bar, reader := progressBarForReader(res.Stderr(), res.Output().(io.Reader), int64(res.Length()))
		bar.Start()

		res.SetOutput(reader)
	},
}

func cat(ctx context.Context, node *core.IpfsNode, paths []string) ([]io.Reader, uint64, error) {
	readers := make([]io.Reader, 0, len(paths))
	length := uint64(0)
	for _, fpath := range paths {
		read, err := coreunix.Cat(ctx, node, fpath)
		if err != nil {
			return nil, 0, err
		}
		readers = append(readers, read)
		length += uint64(read.Size())
	}
	return readers, length, nil
}
