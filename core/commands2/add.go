package commands

import (
	"errors"
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	internal "github.com/jbenet/go-ipfs/core/commands2/internal"
	importer "github.com/jbenet/go-ipfs/importer"
	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	pinning "github.com/jbenet/go-ipfs/pin"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

type AddOutput struct {
	Added []*Object
}

var addCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"recursive", "r"}, cmds.Bool, "Must be specified when adding directories"},
	},
	Arguments: []cmds.Argument{
		cmds.Argument{"file", cmds.ArgFile, true, true, "The path to a file to be added to IPFS"},
	},
	Description: "Add an object to ipfs.",
	Help: `Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run: func(req cmds.Request) (interface{}, error) {
		n := req.Context().Node

		readers, err := internal.CastToReaders(req.Arguments())
		if err != nil {
			return nil, err
		}

		dagnodes, err := add(n, readers)
		if err != nil {
			return nil, errors.New("cast error")
		}

		// TODO: include fs paths in output (will need a way to specify paths in underlying filearg system)
		added := make([]*Object, 0, len(req.Arguments()))
		for _, dagnode := range dagnodes {
			object, err := getOutput(dagnode)
			if err != nil {
				return nil, err
			}

			added = append(added, object)
		}

		return &AddOutput{added}, nil
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
	mp, ok := n.Pinning.(pinning.ManualPinner)
	if !ok {
		return nil, errors.New("invalid pinner type! expected manual pinner")
	}

	dagnodes := make([]*dag.Node, 0)

	// TODO: allow adding directories (will need support for multiple files in filearg system)

	for _, reader := range readers {
		node, err := importer.BuildDagFromReader(reader, n.DAG, mp, chunk.DefaultSplitter)
		if err != nil {
			return nil, err
		}

		err = addNode(n, node)
		if err != nil {
			return nil, err
		}

		dagnodes = append(dagnodes, node)
	}

	return dagnodes, nil
}

func addNode(n *core.IpfsNode, node *dag.Node) error {
	err := n.DAG.AddRecursive(node) // add the file to the graph + local storage
	if err != nil {
		return err
	}

	err = n.Pinning.Pin(node, true) // ensure we keep it
	if err != nil {
		return err
	}

	return nil
}
