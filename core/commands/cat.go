package commands

import (
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"
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
		if res.Length() < progressBarMinSize {
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
		dagnode, err := core.Resolve(ctx, node, path.Path(fpath))
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
