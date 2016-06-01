package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	unixfspb "github.com/ipfs/go-ipfs/unixfs/pb"
)

type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfspb.Data_DataType
}

type LsObject struct {
	Hash  string
	Links []LsLink
}

type LsOutput struct {
	Objects []LsObject
}

var LsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List links from an object.",
		ShortDescription: `
Displays the links an IPFS or IPNS object(s) contains, with the following
format:

  <link base58 hash> <link size in bytes> <link name>
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("headers", "v", "Print table headers (Hash, Size, Name).").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option("headers").Bool(); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		paths := req.Arguments()

		var dagnodes []*merkledag.Node
		for _, fpath := range paths {
			dagnode, err := core.Resolve(req.Context(), node, path.Path(fpath))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]LsObject, len(req.Arguments()))
		for i, dagnode := range dagnodes {
			output[i] = LsObject{
				Hash:  paths[i],
				Links: make([]LsLink, len(dagnode.Links)),
			}
			for j, link := range dagnode.Links {
				var linkNode *merkledag.Node
				linkNode, err = link.GetNode(req.Context(), node.DAG)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				d, err := unixfs.FromBytes(linkNode.Data)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				output[i].Links[j] = LsLink{
					Name: link.Name,
					Hash: link.Hash.B58String(),
					Size: link.Size,
					Type: d.GetType(),
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {

			headers, _, _ := res.Request().Option("headers").Bool()
			output := res.Output().(*LsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, object := range output.Objects {
				if len(output.Objects) > 1 {
					fmt.Fprintf(w, "%s:\n", object.Hash)
				}
				if headers {
					fmt.Fprintln(w, "Hash\tSize\tName")
				}
				for _, link := range object.Links {
					if link.Type == unixfspb.Data_Directory {
						link.Name += "/"
					}
					fmt.Fprintf(w, "%s\t%v\t%s\n", link.Hash, link.Size, link.Name)
				}
				if len(output.Objects) > 1 {
					fmt.Fprintln(w)
				}
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: LsOutput{},
}
