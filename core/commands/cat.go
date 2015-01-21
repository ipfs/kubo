package commands

import (
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
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

		readers := make([]io.Reader, 0, len(req.Arguments()))

		readers, err = cat(node, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		reader := io.MultiReader(readers...)
		res.SetOutput(reader)
	},
}

func cat(node *core.IpfsNode, paths []string) ([]io.Reader, error) {
	readers := make([]io.Reader, 0, len(paths))
	for _, path := range paths {
		dagnode, err := node.Resolver.ResolvePath(path)
		if err != nil {
			return nil, err
		}
		read, err := uio.NewDagReader(dagnode, node.DAG)
		if err != nil {
			return nil, err
		}
		readers = append(readers, read)
	}
	return readers, nil
}
