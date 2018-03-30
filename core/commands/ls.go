package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	blockservice "github.com/ipfs/go-ipfs/blockservice"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	resolver "github.com/ipfs/go-ipfs/path/resolver"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	unixfspb "github.com/ipfs/go-ipfs/unixfs/pb"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
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

const (
	resolveLocal  = "local"
	resolveAlways = "always"
	resolveNever  = "never"
)

var errInvalidResolveType = fmt.Errorf("type must be '%s', '%s' or '%s'", resolveAlways, resolveLocal, resolveNever)

var LsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path, with
the following format:

  <link base58 hash> <link size in bytes> <link name>

The JSON output contains type information.

The '--resolve' option specifies whether to include the type of each linked object.
If 'local', types are only determined for hashes that are pinned or in your local cache.
Note: setting it to 'always' can be VERY slow.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption("resolve-type", "Resolve linked objects to find out their types. Note: this option is deprecated, use --type instead."),
		cmdkit.BoolOption(quietOptionName, "q", "Write only names."),
		cmdkit.StringOption("resolve", "Include the type of each linked object (always|local|never).").WithDefault(resolveAlways),
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

		rtype, _, _ := req.Option("resolve").String()
		if rtype != resolveAlways && rtype != resolveLocal && rtype != resolveNever {
			res.SetError(errInvalidResolveType, cmdkit.ErrClient)
			return
		}
		resolve, defined, err := req.Option("resolve-type").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if defined {
			if resolve {
				rtype = resolveAlways
			} else {
				rtype = resolveLocal
			}
		}

		dserv := nd.DAG
		if rtype == resolveLocal {
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

		quiet, _, _ := req.Option(quietOptionName).Bool()
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
					if rtype == resolveNever {
						break
					}
					linkNode, err := link.GetNode(req.Context(), dserv)
					if err == ipld.ErrNotFound && rtype == resolveLocal {
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
					Size: link.Size,
					Type: t,
				}
				if !quiet {
					output[i].Links[j].Hash = link.Cid.String()
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

			quiet, _, _ := res.Request().Option(quietOptionName).Bool()
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
					if quiet {
						fmt.Fprintln(w, "Name")
					} else {
						fmt.Fprintln(w, "Hash\tSize\tName")
					}
				}
				for _, link := range object.Links {
					if link.Type == unixfspb.Data_Directory {
						link.Name += "/"
					}
					if quiet {
						fmt.Fprintln(w, link.Name)
					} else {
						fmt.Fprintf(w, "%s\t%v\t%s\n", link.Hash, link.Size, link.Name)
					}
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
