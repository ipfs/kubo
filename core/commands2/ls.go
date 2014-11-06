package commands

import (
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
)

type Link struct {
	Name, Hash string
	Size       uint64
}

type Object struct {
	Hash  string
	Links []Link
}

type LsOutput struct {
	Objects []Object
}

var lsCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"object", cmds.ArgString, false, true},
	},
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		node := req.Context().Node

		paths := make([]string, 0)
		for _, arg := range req.Arguments() {
			path, ok := arg.(string)
			if !ok {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}
			paths = append(paths, path)
		}

		dagnodes := make([]*merkledag.Node, 0)
		for _, path := range paths {
			dagnode, err := node.Resolver.ResolvePath(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]Object, len(req.Arguments()))
		for i, dagnode := range dagnodes {
			output[i] = Object{
				Hash:  paths[i],
				Links: make([]Link, len(dagnode.Links)),
			}
			for j, link := range dagnode.Links {
				output[i].Links[j] = Link{
					Name: link.Name,
					Hash: link.Hash.B58String(),
					Size: link.Size,
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			s := ""
			output := res.Output().(*LsOutput).Objects

			for _, object := range output {
				if len(output) > 1 {
					s += fmt.Sprintf("%s:\n", object.Hash)
				}

				for _, link := range object.Links {
					s += fmt.Sprintf("-> %s %s (%v bytes)\n", link.Name, link.Hash, link.Size)
				}

				if len(output) > 1 {
					s += "\n"
				}
			}

			return []byte(s), nil
		},
	},
	Type: &LsOutput{},
}
