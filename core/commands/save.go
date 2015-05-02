package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	nsfs "github.com/ipfs/go-ipfs/ipnsfs"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

var SaveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Save a hash to a location within your ipns keyspace",
		ShortDescription: ``,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("object", true, false, "The object to save"),
		cmds.StringArg("path", true, false, "The path within ipns/local to save the object"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		p, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		obj, err := core.Resolve(nd, p)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = saveNodeTo(nd, obj, req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		k, err := obj.Key()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&SavedEntry{
			Entry: k.Pretty(),
			Path:  req.Arguments()[1],
		})
	},
	Type: SavedEntry{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			se, ok := res.Output().(*SavedEntry)
			if !ok {
				return nil, cmds.ErrIncorrectType
			}

			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "save %s to %s\n", se.Entry, se.Path)
			return buf, nil
		},
	},
}

type SavedEntry struct {
	Entry string
	Path  string
}

func saveNodeTo(nd *core.IpfsNode, obj *dag.Node, target string) error {
	root, err := nd.IpnsFs.GetRoot(nd.Identity.Pretty())
	if err != nil {
		return err
	}

	rdir, ok := root.GetValue().(*nsfs.Directory)
	if !ok {
		return errors.New("your ipns entry doesnt point to a directory")
	}

	parts := strings.Split(target, "/")

	cur := rdir
	for i := 0; i < len(parts)-1; i++ {
		child, err := cur.Child(parts[i])
		if err != nil {
			return err
		}

		switch child := child.(type) {
		case *nsfs.Directory:
			cur = child
		case *nsfs.File:
			return fmt.Errorf("%s is a file", strings.Join(parts[:i+1], "/"))
		}
	}

	err = cur.AddChild(parts[len(parts)-1], obj)
	if err != nil {
		return err
	}

	return nil
}
