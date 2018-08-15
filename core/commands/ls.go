package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	merkledag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	blockservice "gx/ipfs/QmTZZrpd9o4vpYr9TEADW2EoJ9fzUtAgpXqjxZHbKR2T15/go-blockservice"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"
	resolver "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path/resolver"
	unixfs "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs"
	uio "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/io"
	unixfspb "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/pb"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	offline "gx/ipfs/QmVozMmsgK2PYyaHQsrcWLBYigb1m6mW8YhCBG2Cb4Uxq9/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
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
	Helptext: cmdkit.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path, with
the following format:

  <link base58 hash> <link size in bytes> <link name>

The JSON output contains type information.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption("resolve-type", "Resolve linked objects to find out their types.").WithDefault(true),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option("headers").Bool(); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		resolve, _, err := req.Option("resolve-type").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dserv := nd.DAG
		if !resolve {
			offlineexch := offline.Exchange(nd.Blockstore)
			bserv := blockservice.New(nd.Blockstore, offlineexch)
			dserv = merkledag.NewDAGService(bserv)
		}

		paths := req.Arguments()

		var dagnodes []ipld.Node
		for _, fpath := range paths {
			p, err := path.ParsePath(fpath)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			r := &resolver.Resolver{
				DAG:         nd.DAG,
				ResolveOnce: uio.ResolveUnixfsOnce,
			}

			dagnode, err := core.Resolve(req.Context(), nd.Namesys, r, p)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]LsObject, len(req.Arguments()))

		for i, dagnode := range dagnodes {
			dir, err := uio.NewDirectoryFromNode(nd.DAG, dagnode)
			if err != nil && err != uio.ErrNotADir {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			var links []*ipld.Link
			if dir == nil {
				links = dagnode.Links()
			} else {
				links, err = dir.Links(req.Context())
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
			}

			output[i] = LsObject{
				Hash:  paths[i],
				Links: make([]LsLink, len(links)),
			}

			for j, link := range links {
				t := unixfspb.Data_DataType(-1)

				switch link.Cid.Type() {
				case cid.Raw:
					// No need to check with raw leaves
					t = unixfspb.Data_File
				case cid.DagProtobuf:
					linkNode, err := link.GetNode(req.Context(), dserv)
					if err == ipld.ErrNotFound && !resolve {
						// not an error
						linkNode = nil
					} else if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}

					if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
						d, err := unixfs.FromBytes(pn.Data())
						if err != nil {
							res.SetError(err, cmdkit.ErrNormal)
							return
						}
						t = d.GetType()
					}
				}
				output[i].Links[j] = LsLink{
					Name: link.Name,
					Hash: link.Cid.String(),
					Size: link.Size,
					Type: t,
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {

			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			headers, _, _ := res.Request().Option("headers").Bool()
			output, ok := v.(*LsOutput)
			if !ok {
				return nil, e.TypeErr(output, v)
			}

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
