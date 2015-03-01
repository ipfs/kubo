package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/jbenet/go-ipfs/commands"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	path "github.com/jbenet/go-ipfs/path"
	"github.com/jbenet/go-ipfs/unixfs"
	unixfspb "github.com/jbenet/go-ipfs/unixfs/pb"
)

type Link struct {
	Name, Hash string
	Size       uint64
	IsDir      bool
}

type Object struct {
	Hash  string
	Links []Link
}

type LsOutput struct {
	Objects []Object
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

		output := make([]Object, len(req.Arguments()))
		for i, dagnode := range dagnodes {
			output[i] = Object{
				Hash:  paths[i],
				Links: make([]Link, len(dagnode.Links)),
			}
			for j, link := range dagnode.Links {
				link.Node, err = link.GetNode(node.DAG)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				d, err := unixfs.FromBytes(link.Node.Data)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				output[i].Links[j] = Link{
					Name:  link.Name,
					Hash:  link.Hash.B58String(),
					Size:  link.Size,
					IsDir: d.GetType() == unixfspb.Data_Directory,
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			output := res.Output().(*LsOutput).Objects
			var buf bytes.Buffer
			w := tabwriter.NewWriter(&buf, 1, 2, 1, ' ', 0)
			for _, object := range output {
				if len(output) > 1 {
					fmt.Fprintf(w, "%s:\n", object.Hash)
				}
				marshalLinks(w, object.Links)
				if len(output) > 1 {
					fmt.Fprintln(w)
				}
			}
			w.Flush()

			return &buf, nil
		},
	},
	Type: LsOutput{},
}

func marshalLinks(w io.Writer, links []Link) {
	fmt.Fprintln(w, "Hash\tSize\tName\t")
	for _, link := range links {
		if link.IsDir {
			link.Name += "/"
		}
		fmt.Fprintf(w, "%s\t%v\t%s\t\n", link.Hash, link.Size, link.Name)
	}
}
