package unixfs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
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
	Type       string
}

type LsObject struct {
	Links []LsLink
}

type LsOutput struct {
	Arguments map[string]string
	Objects   map[string]*LsObject
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

		output := LsOutput{
			Arguments: map[string]string{},
			Objects:   map[string]*LsObject{},
		}

		for _, fpath := range paths {
			ctx := req.Context().Context
			merkleNode, err := core.Resolve(ctx, node, path.Path(fpath))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			key, err := merkleNode.Key()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			hash := key.B58String()
			output.Arguments[fpath] = hash

			if _, ok := output.Objects[hash]; ok {
				// duplicate argument for an already-listed node
				continue
			}

			output.Objects[hash] = &LsObject{}

			unixFSNode, err := unixfs.FromBytes(merkleNode.Data)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			t := unixFSNode.GetType()
			switch t {
			default:
				res.SetError(fmt.Errorf("unrecognized type: %s", t), cmds.ErrImplementation)
				return
			case unixfspb.Data_File:
				key, err := merkleNode.Key()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				output.Objects[hash].Links = []LsLink{LsLink{
					Name: fpath,
					Hash: key.String(),
					Type: t.String(),
					Size: unixFSNode.GetFilesize(),
				}}
			case unixfspb.Data_Directory:
				links := make([]LsLink, len(merkleNode.Links))
				output.Objects[hash].Links = links
				for i, link := range merkleNode.Links {
					getCtx, cancel := context.WithTimeout(ctx, time.Minute)
					defer cancel()
					link.Node, err = link.GetNode(getCtx, node.DAG)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}
					d, err := unixfs.FromBytes(link.Node.Data)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}
					t := d.GetType()
					lsLink := LsLink{
						Name: link.Name,
						Hash: link.Hash.B58String(),
						Type: t.String(),
					}
					if t == unixfspb.Data_File {
						lsLink.Size = d.GetFilesize()
					} else {
						lsLink.Size = link.Size
					}
					links[i] = lsLink
				}
			}
		}

		res.SetOutput(&output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {

			output := res.Output().(*LsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)

			nonDirectories := []string{}
			directories := []string{}
			for argument, hash := range output.Arguments {
				object, ok := output.Objects[hash]
				if !ok {
					return nil, fmt.Errorf("unresolved hash: %s", hash)
				}

				if len(object.Links) == 1 && object.Links[0].Hash == hash {
					nonDirectories = append(nonDirectories, argument)
				} else {
					directories = append(directories, argument)
				}
			}
			sort.Strings(nonDirectories)
			sort.Strings(directories)

			for _, argument := range nonDirectories {
				fmt.Fprintf(w, "%s\n", argument)
			}

			seen := map[string]bool{}
			for i, argument := range directories {
				hash := output.Arguments[argument]
				if _, ok := seen[hash]; ok {
					continue
				}
				seen[hash] = true

				object := output.Objects[hash]
				if i > 0 || len(nonDirectories) > 0 {
					fmt.Fprintln(w)
				}
				if len(output.Arguments) > 1 {
					for _, arg := range directories[i:] {
						if output.Arguments[arg] == hash {
							fmt.Fprintf(w, "%s:\n", arg)
						}
					}
				}
				for _, link := range object.Links {
					fmt.Fprintf(w, "%s\n", link.Name)
				}
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: LsOutput{},
}
