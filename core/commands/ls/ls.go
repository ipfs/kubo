package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	ccutil "github.com/jbenet/go-ipfs/core/commands/util"
	path "github.com/jbenet/go-ipfs/path"
	merkledag "github.com/jbenet/go-ipfs/struct/merkledag"
)

type LsOutput struct {
	Objects []ccutil.Object
}

var LsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List links from an object.",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and displays the links
it contains, with the following format:

  <link base58 hash> <link size in bytes> <link name>
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		paths := req.Arguments()

		dagnodes := make([]*merkledag.Node, 0)
		for _, fpath := range paths {
			dagnode, err := node.Resolver.ResolvePath(path.Path(fpath))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]ccutil.Object, len(req.Arguments()))
		for i, dagnode := range dagnodes {
			output[i] = ccutil.Object{
				Hash:  paths[i],
				Links: make([]ccutil.Link, len(dagnode.Links)),
			}
			for j, link := range dagnode.Links {
				output[i].Links[j] = ccutil.Link{
					Name: link.Name,
					Hash: link.Hash.B58String(),
					Size: link.Size,
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			s := ""
			output := res.Output().(*LsOutput).Objects

			for _, object := range output {
				if len(output) > 1 {
					s += fmt.Sprintf("%s:\n", object.Hash)
				}
				s += ccutil.MarshalLinks(object.Links)
				if len(output) > 1 {
					s += "\n"
				}
			}

			return strings.NewReader(s), nil
		},
	},
	Type: LsOutput{},
}
