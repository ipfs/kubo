package unixfs

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	merkledag "github.com/ipfs/go-merkledag"
	unixfs "github.com/ipfs/go-unixfs"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       string
}

type LsObject struct {
	Hash  string
	Size  uint64
	Type  string
	Links []LsLink
}

type LsOutput struct {
	Arguments map[string]string
	Objects   map[string]*LsObject
}

var LsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path.

The JSON output contains size information. For files, the child size
is the total size of the file contents. For directories, the child
size is the IPFS link size.

This functionality is deprecated, and will be removed in future versions. If
possible, please use 'ipfs ls' instead.
`,
		LongDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path.

The JSON output contains size information. For files, the child size
is the total size of the file contents. For directories, the child
size is the IPFS link size.

The path can be a prefixless ref; in this case, we assume it to be an
/ipfs ref and not /ipns.

Example:

    > ipfs file ls QmW2WQi7j6c7UgJTarActp7tDNikE4B2qXtFCfLPdsgaTQ
    cat.jpg
    > ipfs file ls /ipfs/QmW2WQi7j6c7UgJTarActp7tDNikE4B2qXtFCfLPdsgaTQ
    cat.jpg

This functionality is deprecated, and will be removed in future versions. If
possible, please use 'ipfs ls' instead.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		paths := req.Arguments

		output := LsOutput{
			Arguments: map[string]string{},
			Objects:   map[string]*LsObject{},
		}

		for _, p := range paths {
			ctx := req.Context

			merkleNode, err := api.ResolveNode(ctx, path.New(p))
			if err != nil {
				return err
			}

			c := merkleNode.Cid()

			hash := c.String()
			output.Arguments[p] = hash

			if _, ok := output.Objects[hash]; ok {
				// duplicate argument for an already-listed node
				continue
			}

			ndpb, ok := merkleNode.(*merkledag.ProtoNode)
			if !ok {
				return merkledag.ErrNotProtobuf
			}

			unixFSNode, err := unixfs.FSNodeFromBytes(ndpb.Data())
			if err != nil {
				return err
			}

			t := unixFSNode.Type()

			output.Objects[hash] = &LsObject{
				Hash: c.String(),
				Type: t.String(),
				Size: unixFSNode.FileSize(),
			}

			switch t {
			case unixfs.TFile:
				break
			case unixfs.THAMTShard:
				// We need a streaming ls API for this.
				return fmt.Errorf("cannot list large directories yet")
			case unixfs.TDirectory:
				links := make([]LsLink, len(merkleNode.Links()))
				output.Objects[hash].Links = links
				for i, link := range merkleNode.Links() {
					linkNode, err := link.GetNode(ctx, nd.DAG)
					if err != nil {
						return err
					}
					lnpb, ok := linkNode.(*merkledag.ProtoNode)
					if !ok {
						return merkledag.ErrNotProtobuf
					}

					d, err := unixfs.FSNodeFromBytes(lnpb.Data())
					if err != nil {
						return err
					}
					t := d.Type()
					lsLink := LsLink{
						Name: link.Name,
						Hash: link.Cid.String(),
						Type: t.String(),
					}
					if t == unixfs.TFile {
						lsLink.Size = d.FileSize()
					} else {
						lsLink.Size = link.Size
					}
					links[i] = lsLink
				}
			case unixfs.TSymlink:
				return fmt.Errorf("cannot list symlinks yet")
			default:
				return fmt.Errorf("unrecognized type: %s", t)
			}
		}

		return cmds.EmitOnce(res, &output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *LsOutput) error {
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)

			nonDirectories := []string{}
			directories := []string{}
			for argument, hash := range out.Arguments {
				object, ok := out.Objects[hash]
				if !ok {
					return fmt.Errorf("unresolved hash: %s", hash)
				}

				if object.Type == "Directory" {
					directories = append(directories, argument)
				} else {
					nonDirectories = append(nonDirectories, argument)
				}
			}
			sort.Strings(nonDirectories)
			sort.Strings(directories)

			for _, argument := range nonDirectories {
				fmt.Fprintf(tw, "%s\n", argument)
			}

			seen := map[string]bool{}
			for i, argument := range directories {
				hash := out.Arguments[argument]
				if _, ok := seen[hash]; ok {
					continue
				}
				seen[hash] = true

				object := out.Objects[hash]
				if i > 0 || len(nonDirectories) > 0 {
					fmt.Fprintln(tw)
				}
				if len(out.Arguments) > 1 {
					for _, arg := range directories[i:] {
						if out.Arguments[arg] == hash {
							fmt.Fprintf(tw, "%s:\n", arg)
						}
					}
				}
				for _, link := range object.Links {
					fmt.Fprintf(tw, "%s\n", link.Name)
				}
			}
			tw.Flush()

			return nil
		}),
	},
	Type: LsOutput{},
}
