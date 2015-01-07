package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	pinning "github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

type AddedObject struct {
	Name string
	Hash string
}

var AddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add an object to ipfs.",
		ShortDescription: `
Adds contents of <path> to ipfs. Use -r to add directories.
Note that directories are added recursively, to form the ipfs
MerkleDAG. A smarter partial add with a staging area (like git)
remains to be implemented.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path to a file to be added to IPFS").EnableRecursive(),
	},
	Options: []cmds.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmds.BoolOption("quiet", "q", "Write minimal output"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		outChan := make(chan interface{})

		go func() {
			defer close(outChan)

			for {
				file, err := req.Files().NextFile()
				if (err != nil && err != io.EOF) || file == nil {
					return
				}

				_, err = addFile(n, file, outChan)
				if err != nil {
					return
				}
			}
		}()

		return outChan, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			quiet, _, err := res.Request().Option("quiet").Bool()
			if err != nil {
				return nil, err
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*AddedObject)
				if !ok {
					return nil, u.ErrCast()
				}

				var buf bytes.Buffer
				if quiet {
					buf.WriteString(fmt.Sprintf("%s\n", obj.Hash))
				} else {
					buf.WriteString(fmt.Sprintf("added %s %s\n", obj.Hash, obj.Name))
				}
				return &buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
			}, nil
		},
	},
	Type: AddedObject{},
}

func add(n *core.IpfsNode, readers []io.Reader) ([]*dag.Node, error) {
	mp, ok := n.Pinning.(pinning.ManualPinner)
	if !ok {
		return nil, errors.New("invalid pinner type! expected manual pinner")
	}

	dagnodes := make([]*dag.Node, 0)

	for _, reader := range readers {
		node, err := importer.BuildDagFromReader(reader, n.DAG, mp, chunk.DefaultSplitter)
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

func addFile(n *core.IpfsNode, file cmds.File, out chan interface{}) (*dag.Node, error) {
	if file.IsDirectory() {
		return addDir(n, file, out)
	}

	dns, err := add(n, []io.Reader{file})
	if err != nil {
		return nil, err
	}

	log.Infof("adding file: %s", file.FileName())
	if err := outputDagnode(out, file.FileName(), dns[len(dns)-1]); err != nil {
		return nil, err
	}
	return dns[len(dns)-1], nil // last dag node is the file.
}

func addDir(n *core.IpfsNode, dir cmds.File, out chan interface{}) (*dag.Node, error) {
	log.Infof("adding directory: %s", dir.FileName())

	tree := &dag.Node{Data: ft.FolderPBData()}

	for {
		file, err := dir.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := addFile(n, file, out)
		if err != nil {
			return nil, err
		}

		_, name := path.Split(file.FileName())

		err = tree.AddNodeLink(name, node)
		if err != nil {
			return nil, err
		}
	}

	err := outputDagnode(out, dir.FileName(), tree)
	if err != nil {
		return nil, err
	}

	err = addNode(n, tree)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan interface{}, name string, dn *dag.Node) error {
	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &AddedObject{
		Hash: o.Hash,
		Name: name,
	}

	return nil
}
