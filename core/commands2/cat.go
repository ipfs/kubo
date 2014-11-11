package commands

import (
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/commands2/internal"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

var catCmd = &cmds.Command{
	Description: "Show IPFS object data",
	Help: `Retrieves the object named by <ipfs-path> and outputs the data
it contains.
	`,

	Arguments: []cmds.Argument{
		cmds.Argument{"ipfs-path", cmds.ArgString, true, true,
			"The path to the IPFS object(s) to be outputted"},
	},
	Run: func(req cmds.Request) (interface{}, error) {
		node := req.Context().Node
		readers := make([]io.Reader, 0, len(req.Arguments()))

		paths, err := internal.CastToStrings(req.Arguments())
		if err != nil {
			return nil, err
		}

		readers, err = cat(node, paths)
		if err != nil {
			return nil, err
		}

		reader := io.MultiReader(readers...)
		return reader, nil
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
