package commands

import (
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

var cat = &cmds.Command{
	Help: "TODO",
	Run: func(req cmds.Request, res cmds.Response) {
		node := req.Context().Node
		fmt.Println(node.Resolver)
		readers := make([]io.Reader, 0, len(req.Arguments()))

		for _, path := range req.Arguments() {
			dagnode, err := node.Resolver.ResolvePath(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			read, err := uio.NewDagReader(dagnode, node.DAG)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			readers = append(readers, read)
		}

		reader := io.MultiReader(readers...)
		res.SetValue(reader)
	},
}
