package commands

import (
	"errors"
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	internal "github.com/jbenet/go-ipfs/core/commands2/internal"
	importer "github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

type AddOutput struct {
	Added []Object
}

var addCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"recursive", "r"}, cmds.Bool},
	},
	Arguments: []cmds.Argument{
		cmds.Argument{"file", cmds.ArgFile, false, true},
	},
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		// if recursive, set depth to reflect so
		// opt, found := req.Option("r")
		// if r, _ := opt.(bool); found && r {
		// }

		readers, err := internal.ToReaders(req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dagnodes, err := add(n, readers)
		if err != nil {
			res.SetError(errors.New("cast error"), cmds.ErrNormal)
			return
		}

		added := make([]Object, len(req.Arguments()))
		for _, dagnode := range dagnodes {

			k, err := dagnode.Key()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			added = append(added, Object{Hash: k.String(), Links: nil})
		}

		res.SetOutput(&AddOutput{added})
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			val, ok := res.Output().(*AddOutput)
			if !ok {
				return nil, errors.New("cast err")
			}
			added := val.Added
			if len(added) == 1 {
				s := fmt.Sprintf("Added object: %s\n", added[0].Hash)
				return []byte(s), nil
			}

			s := fmt.Sprintf("Added %v objects:\n", len(added))
			for _, obj := range added {
				s += fmt.Sprintf("- %s\n", obj.Hash)
			}
			return []byte(s), nil
		},
	},
	Type: &AddOutput{},
}

func add(n *core.IpfsNode, readers []io.Reader) ([]*dag.Node, error) {

	dagnodes := make([]*dag.Node, 0)

	for _, reader := range readers {
		node, err := importer.NewDagFromReader(reader)
		if err != nil {
			return nil, err
		}

		err = n.DAG.AddRecursive(node) // add the file to the graph + local storage
		if err != nil {
			return nil, err
		}

		err = n.Pinning.Pin(node, true) // ensure we keep it
		if err != nil {
			return nil, err
		}

		dagnodes = append(dagnodes, node)
	}
	return dagnodes, nil
}
