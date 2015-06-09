package unixfs

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
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
	Argument string
	Links    []LsLink
}

type LsOutput struct {
	Objects []*LsObject
}

var LsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directory contents for Unix-filesystem objects",
		ShortDescription: `
Retrieves the object named by <ipfs-or-ipns-path> and displays the
contents with the following format:

  <hash> <type> <size> <name>

For files, the child size is the total size of the file contents.  For
directories, the child size is the IPFS link size.
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

		output := make([]*LsObject, len(paths))
		for i, fpath := range paths {
			dagnode, err := core.Resolve(req.Context().Context, node, path.Path(fpath))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			output[i] = &LsObject{
				Argument: fpath,
				Links:    make([]LsLink, len(dagnode.Links)),
			}
			for j, link := range dagnode.Links {
				ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
				defer cancel()
				link.Node, err = link.GetNode(ctx, node.DAG)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				d, err := unixfs.FromBytes(link.Node.Data)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				lsLink := LsLink{
					Name: link.Name,
					Hash: link.Hash.B58String(),
					Type: d.GetType(),
				}
				if lsLink.Type == unixfspb.Data_File {
					lsLink.Size = d.GetFilesize()
				} else {
					lsLink.Size = link.Size
				}
				output[i].Links[j] = lsLink
			}
		}

		res.SetOutput(&LsOutput{Objects: output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {

			output := res.Output().(*LsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, object := range output.Objects {
				if len(output.Objects) > 1 {
					fmt.Fprintf(w, "%s:\n", object.Argument)
				}
				for _, link := range object.Links {
					fmt.Fprintf(w, "%s\n", link.Name)
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
