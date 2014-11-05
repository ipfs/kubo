package commands

import (
	"errors"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

var catCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"object", cmds.ArgString, true, true},
	},
	Help: `ipfs cat <object> - Show ipfs object data.

	Retrieves the object named by <object> and outputs the data
	it contains.
	`,
	Run: func(res cmds.Response, req cmds.Request) {
		node := req.Context().Node
		paths := make([]string, 0, len(req.Arguments()))
		readers := make([]io.Reader, 0, len(req.Arguments()))

		for _, arg := range req.Arguments() {
			path, ok := arg.(string)
			if !ok {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}
			paths = append(paths, path)
		}

		readers, err := cat(node, paths)
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
