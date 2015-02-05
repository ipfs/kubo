package commands

import (
	"io"

	cmds "github.com/jbenet/go-ipfs/thirdparty/commands"
	core "github.com/jbenet/go-ipfs/core"
	ccutil "github.com/jbenet/go-ipfs/core/commands/util"
	path "github.com/jbenet/go-ipfs/path"
	uio "github.com/jbenet/go-ipfs/unixfs/io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"
)

var CatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show IPFS object data",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and outputs the data
it contains.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		readers, length, err := cat(req.Context().Context, node, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetLength(length)

		reader := io.MultiReader(readers...)
		res.SetOutput(reader)
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Length() < ccutil.ProgressBarMinSize {
			return
		}

		bar := pb.New(int(res.Length())).SetUnits(pb.U_BYTES)
		bar.Output = res.Stderr()
		bar.Start()

		reader := bar.NewProxyReader(res.Output().(io.Reader))
		res.SetOutput(reader)
	},
}

func cat(ctx context.Context, node *core.IpfsNode, paths []string) ([]io.Reader, uint64, error) {
	readers := make([]io.Reader, 0, len(paths))
	length := uint64(0)
	for _, fpath := range paths {
		dagnode, err := node.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, 0, err
		}

		read, err := uio.NewDagReader(ctx, dagnode, node.DAG)
		if err != nil {
			return nil, 0, err
		}
		readers = append(readers, read)
		length += uint64(read.Size())
	}
	return readers, length, nil
}
