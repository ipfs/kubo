package commands

import (
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
	Helptext: cmds.HelpText{
		Tagline: "List links from an object.",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and displays the links
it contains, with the following format:

  <link base58 hash> <link size in bytes> <link name>
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		node, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		paths := req.Arguments()

		dagnodes := make([]*merkledag.Node, 0)
		for _, path := range paths {
			dagnode, err := node.Resolver.ResolvePath(path)
			if err != nil {
				return nil, err
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

		return &LsOutput{output}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			s := ""
			output := res.Output().(*LsOutput).Objects

			for _, object := range output {
				if len(output) > 1 {
					s += fmt.Sprintf("%s:\n", object.Hash)
				}
				s += marshalLinks(object.Links)
				if len(output) > 1 {
					s += "\n"
				}
			}

			return []byte(s), nil
		},
	},
	Type: &LsOutput{},
}

func marshalLinks(links []Link) (s string) {
	for _, link := range links {
		s += fmt.Sprintf("%s %v %s\n", link.Hash, link.Size, link.Name)
	}
	return s
}
