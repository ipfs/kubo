package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	iface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	offline "gx/ipfs/QmT6dHGp3UYd3vUMpy7rzX2CXQv7HLcj42Vtq8qwwjgASb/go-ipfs-exchange-offline"
	unixfs "gx/ipfs/QmTJUySFxXjh54zEoFbzQEmGD3yj89XKS3A28y7Nqsn1TC/go-unixfs"
	uio "gx/ipfs/QmTJUySFxXjh54zEoFbzQEmGD3yj89XKS3A28y7Nqsn1TC/go-unixfs/io"
	unixfspb "gx/ipfs/QmTJUySFxXjh54zEoFbzQEmGD3yj89XKS3A28y7Nqsn1TC/go-unixfs/pb"
	blockservice "gx/ipfs/QmWfhv1D18DRSiSm73r4QGcByspzPtxxRTcmHW3axFXZo8/go-blockservice"
	merkledag "gx/ipfs/QmY8BMUSpCwNiTmFhACmC9Bt1qT63cHP35AoQAus4x14qH/go-merkledag"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
	cmds "gx/ipfs/QmdTmGruUz23vgzym3uWpnAEQdGdGifQqBvP8UXSRjG8gZ/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
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
	lsHeadersOptionNameTime = "headers"
	lsResolveTypeOptionName = "resolve-type"
)

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
		cmdkit.BoolOption(lsHeadersOptionNameTime, "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption(lsResolveTypeOptionName, "Resolve linked objects to find out their types.").WithDefault(true),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		resolve, _ := req.Options[lsResolveTypeOptionName].(bool)

		dserv := nd.DAG
		if !resolve {
			offlineexch := offline.Exchange(nd.Blockstore)
			bserv := blockservice.New(nd.Blockstore, offlineexch)
			dserv = merkledag.NewDAGService(bserv)
		}

		paths := req.Arguments

		var dagnodes []ipld.Node
		for _, fpath := range paths {
			p, err := iface.ParsePath(fpath)
			if err != nil {
				return err
			}

			dagnode, err := api.ResolveNode(req.Context, p)
			if err != nil {
				return err
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]LsObject, len(req.Arguments))
		ng := merkledag.NewSession(req.Context, nd.DAG)
		ro := merkledag.NewReadOnlyDagService(ng)

		for i, dagnode := range dagnodes {
			dir, err := uio.NewDirectoryFromNode(ro, dagnode)
			if err != nil && err != uio.ErrNotADir {
				return fmt.Errorf("the data in %s (at %q) is not a UnixFS directory: %s", dagnode.Cid(), paths[i], err)
			}

			var links []*ipld.Link
			if dir == nil {
				links = dagnode.Links()
			} else {
				links, err = dir.Links(req.Context)
				if err != nil {
					return err
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
					t = unixfs.TFile
				case cid.DagProtobuf:
					linkNode, err := link.GetNode(req.Context, dserv)
					if err == ipld.ErrNotFound && !resolve {
						// not an error
						linkNode = nil
					} else if err != nil {
						return err
					}

					if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
						d, err := unixfs.FSNodeFromBytes(pn.Data())
						if err != nil {
							return err
						}
						t = d.Type()
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

		return res.Emit(&LsOutput{output})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *LsOutput) error {
			headers, _ := req.Options[lsHeadersOptionNameTime].(bool)

			buf := new(bytes.Buffer)
			tw := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, object := range out.Objects {
				if len(out.Objects) > 1 {
					fmt.Fprintf(tw, "%s:\n", object.Hash)
				}
				if headers {
					fmt.Fprintln(tw, "Hash\tSize\tName")
				}
				for _, link := range object.Links {
					if link.Type == unixfs.TDirectory {
						link.Name += "/"
					}
					fmt.Fprintf(tw, "%s\t%v\t%s\n", link.Hash, link.Size, link.Name)
				}
				if len(out.Objects) > 1 {
					fmt.Fprintln(tw)
				}
			}
			tw.Flush()

			fmt.Fprint(w, buf)

			return nil
		}),
	},
	Type: LsOutput{},
}
