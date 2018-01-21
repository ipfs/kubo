package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strings"

	bservice "github.com/ipfs/go-ipfs/blockservice"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mfs "github.com/ipfs/go-ipfs/mfs"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	cmds "gx/ipfs/QmP9vZfc5WSjfGTXmwX2EcicMFzmZ6fXn7HTdKYat6ccmH/go-ipfs-cmds"
	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	cmdkit "gx/ipfs/QmQp2a2Hhb7F6eK2A5hN8f9aJy4mtkEikL9Zj4cgB7d1dD/go-ipfs-cmdkit"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	mh "gx/ipfs/QmYeKnKpubCMRiq3PGZcTREErthbb5Q9cXsCoSkD9bjEBd/go-multihash"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

var log = logging.Logger("cmds/files")

var FilesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with unixfs files.",
		ShortDescription: `
Files is an API for manipulating IPFS objects as if they were a unix
filesystem.

NOTE:
Most of the subcommands of 'ipfs files' accept the '--flush' flag. It defaults
to true. Use caution when setting this flag to false. It will improve
performance for large numbers of file operations, but it does so at the cost
of consistency guarantees. If the daemon is unexpectedly killed before running
'ipfs files flush' on the files in question, then data may be lost. This also
applies to running 'ipfs repo gc' concurrently with '--flush=false'
operations.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("f", "flush", "Flush target and ancestors after write.").WithDefault(true),
	},
	Subcommands: map[string]*cmds.Command{
		"tree": filesTreeCmd,
	},
	OldSubcommands: map[string]*oldcmds.Command{
		"read":  filesReadCmd,
		"write": filesWriteCmd,
		"mv":    filesMvCmd,
		"cp":    filesCpCmd,
		"ls":    filesLsCmd,
		"mkdir": filesMkdirCmd,
		"stat":  filesStatCmd,
		"rm":    filesRmCmd,
		"flush": filesFlushCmd,
		"chcid": filesChcidCmd,
	},
}

var cidVersionOption = cmdkit.IntOption("cid-version", "cid-ver", "Cid version to use. (experimental)")
var hashOption = cmdkit.StringOption("hash", "Hash function to use. Will set Cid version to 1 if used. (experimental)")

var formatError = errors.New("Format was set by multiple options. Only one format option is allowed")

var filesStatCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Display file status.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to node to stat."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "Print statistics in given format. Allowed tokens: "+
			"<hash> <size> <cumulsize> <type> <childs>. Conflicts with other format options.").WithDefault(
			`<hash>
Size: <size>
CumulativeSize: <cumulsize>
ChildBlocks: <childs>
Type: <type>`),
		cmdkit.BoolOption("hash", "Print only hash. Implies '--format=<hash>'. Conflicts with other format options."),
		cmdkit.BoolOption("size", "Print only size. Implies '--format=<cumulsize>'. Conflicts with other format options."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {

		_, err := statGetFormatOptions(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrClient)
		}

		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(node.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		o, err := statNode(node.DAG, fsn)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(o)
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*Object)
			if !ok {
				return nil, e.TypeErr(out, v)
			}
			buf := new(bytes.Buffer)

			s, _ := statGetFormatOptions(res.Request())
			s = strings.Replace(s, "<hash>", out.Hash, -1)
			s = strings.Replace(s, "<size>", fmt.Sprintf("%d", out.Size), -1)
			s = strings.Replace(s, "<cumulsize>", fmt.Sprintf("%d", out.CumulativeSize), -1)
			s = strings.Replace(s, "<childs>", fmt.Sprintf("%d", out.Blocks), -1)
			s = strings.Replace(s, "<type>", out.Type, -1)

			fmt.Fprintln(buf, s)
			return buf, nil
		},
	},
	Type: Object{},
}

func moreThanOne(a, b, c bool) bool {
	return a && b || b && c || a && c
}

func statGetFormatOptions(req oldcmds.Request) (string, error) {

	hash, _, _ := req.Option("hash").Bool()
	size, _, _ := req.Option("size").Bool()
	format, found, _ := req.Option("format").String()

	if moreThanOne(hash, size, found) {
		return "", formatError
	}

	if hash {
		return "<hash>", nil
	} else if size {
		return "<cumulsize>", nil
	} else {
		return format, nil
	}
}

func statNode(ds dag.DAGService, fsn mfs.FSNode) (*Object, error) {
	nd, err := fsn.GetNode()
	if err != nil {
		return nil, err
	}

	c := nd.Cid()

	cumulsize, err := nd.Size()
	if err != nil {
		return nil, err
	}

	switch n := nd.(type) {
	case *dag.ProtoNode:
		d, err := ft.FromBytes(n.Data())
		if err != nil {
			return nil, err
		}

		var ndtype string
		switch fsn.Type() {
		case mfs.TDir:
			ndtype = "directory"
		case mfs.TFile:
			ndtype = "file"
		default:
			return nil, fmt.Errorf("unrecognized node type: %s", fsn.Type())
		}

		return &Object{
			Hash:           c.String(),
			Blocks:         len(nd.Links()),
			Size:           d.GetFilesize(),
			CumulativeSize: cumulsize,
			Type:           ndtype,
		}, nil
	case *dag.RawNode:
		return &Object{
			Hash:           c.String(),
			Blocks:         0,
			Size:           cumulsize,
			CumulativeSize: cumulsize,
			Type:           "file",
		}, nil
	default:
		return nil, fmt.Errorf("not unixfs node (proto or raw)")
	}
}

var filesCpCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Copy files within mfs.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("source", true, false, "Source object to copy."),
		cmdkit.StringArg("dest", true, false, "Destination to copy object to."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		flush, _, _ := req.Option("flush").Bool()

		src, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		src = strings.TrimRight(src, "/")

		dst, err := checkPath(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if dst[len(dst)-1] == '/' {
			dst += gopath.Base(src)
		}

		nd, err := getNodeFromPath(req.Context(), node, node.DAG, src)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = mfs.PutNode(node.FilesRoot, dst, nd)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if flush {
			err := mfs.FlushPath(node.FilesRoot, dst)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		res.SetOutput(nil)
	},
}

func getNodeFromPath(ctx context.Context, node *core.IpfsNode, dagserv dag.DAGService, p string) (node.Node, error) {
	switch {
	case strings.HasPrefix(p, "/ipfs/"):
		np, err := path.ParsePath(p)
		if err != nil {
			return nil, err
		}

		resolver := &path.Resolver{
			DAG:         dagserv,
			ResolveOnce: uio.ResolveUnixfsOnce,
		}

		return core.Resolve(ctx, node.Namesys, resolver, np)
	default:
		fsn, err := mfs.Lookup(node.FilesRoot, p)
		if err != nil {
			return nil, err
		}

		return fsn.GetNode()
	}
}

type Object struct {
	Hash           string
	Size           uint64
	CumulativeSize uint64
	Blocks         int
	Type           string
}

type filesLsOutput struct {
	Entries []mfs.NodeListing
}

var filesLsCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List directories in the local mutable namespace.",
		ShortDescription: `
List directories in the local mutable namespace.

Examples:

    $ ipfs files ls /welcome/docs/
    about
    contact
    help
    quick-start
    readme
    security-notes

    $ ipfs files ls /myfiles/a/b/c/d
    foo
    bar
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", false, false, "Path to show listing for. Defaults to '/'."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("l", "Use long listing format."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		var arg string

		if len(req.Arguments()) == 0 {
			arg = "/"
		} else {
			arg = req.Arguments()[0]
		}

		path, err := checkPath(arg)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(nd.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		long, _, _ := req.Option("l").Bool()

		switch fsn := fsn.(type) {
		case *mfs.Directory:
			if !long {
				var output []mfs.NodeListing
				names, err := fsn.ListNames(req.Context())
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}

				for _, name := range names {
					output = append(output, mfs.NodeListing{
						Name: name,
					})
				}
				res.SetOutput(&filesLsOutput{output})
			} else {
				listing, err := fsn.List(req.Context())
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
				res.SetOutput(&filesLsOutput{listing})
			}
			return
		case *mfs.File:
			_, name := gopath.Split(path)
			out := &filesLsOutput{[]mfs.NodeListing{mfs.NodeListing{Name: name, Type: 1}}}
			res.SetOutput(out)
			return
		default:
			res.SetError(errors.New("unrecognized type"), cmdkit.ErrNormal)
		}
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*filesLsOutput)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			long, _, _ := res.Request().Option("l").Bool()

			for _, o := range out.Entries {
				if long {
					fmt.Fprintf(buf, "%s\t%s\t%d\n", o.Name, o.Hash, o.Size)
				} else {
					fmt.Fprintf(buf, "%s\n", o.Name)
				}
			}
			return buf, nil
		},
	},
	Type: filesLsOutput{},
}

type treeItem struct {
	Name      string
	Depth     int
	LastChild bool
}

type treeSummary struct {
	Local     bool
	SizeLocal uint64 `json:",omitempty"`
	SizeTotal uint64 `json:",omitempty"`
	NbFile    int    `json:",omitempty"`
	NbDir     int    `json:",omitempty"`
}

const itemType = "item"
const summaryType = "summary"

type treeOutput struct {
	Hash string
	Type string

	Item    treeItem
	Summary treeSummary
}

var filesTreeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show informations about a tree of files",
		ShortDescription: `
'ipfs files tree' display the tree structure of directories/files

The path can be inside of MFS or not.

Examples:

	$ ipfs files tree /ipfs/QmQLXHs7K98JNQdWrBB2cQLJahPhmupbDjRuH1b9ibmwVa
`,
		LongDescription: `
'ipfs files tree' display the tree structure of directories/files

The path can be inside of MFS or not.

Examples:

	$ ipfs files tree /ipfs/QmQLXHs7K98JNQdWrBB2cQLJahPhmupbDjRuH1b9ibmwVa/locale
	QmPA5R3e7FJZpFAT5NYRcYuLJay1qS3enu4zUHAkQMu5uW
	├── webui-cs.json
	├── webui-de.json
	├── webui-en.json
	├── webui-fr.json
	├── webui-nl.json
	├── webui-pl.json
	├── webui-th.json
	└── webui-zh.json
	0 directories, 8 files present
	23 kB present of 23 kB (100.00%)
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to print information about."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("only-summary", "s", "Only print the summary of the tree's information."),
		// TODO: "local" is already used by the root ipfs command, what do ?
		cmdkit.BoolOption("local2", "l", "Don't request data from the network."),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		onlySummary, _, err := req.Option("only-summary").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		local, _, err := req.Option("local2").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var dagserv dag.DAGService
		if local {
			// an offline DAGService will not fetch from the network
			dagserv = dag.NewDAGService(bservice.New(
				nd.Blockstore,
				offline.Exchange(nd.Blockstore),
			))
		} else {
			// regular connected DAGService
			dagserv = nd.DAG
		}

		root, err := getNodeFromPath(req.Context(), nd, dagserv, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		isDirectory := false
		// try to decode unixfs
		if pn, ok := root.(*dag.ProtoNode); ok {
			unixfs, err := ft.FromBytes(pn.Data())
			if err == nil {
				isDirectory = unixfs.GetType() == ft.TDirectory
			}
		}

		summary, err := walkBlockStart(req.Context(), res, dagserv, root, isDirectory, onlySummary)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.Emit(&treeOutput{
			Hash:    root.Cid().String(),
			Type:    summaryType,
			Summary: summary,
		})

	},
	Type: treeOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req cmds.Request, w io.Writer, v interface{}) error {
			output, ok := v.(*treeOutput)
			if !ok {
				return e.TypeErr(output, v)
			}

			if output.Type == itemType {
				item := output.Item

				if item.Depth > 1 {
					fmt.Fprint(w, strings.Repeat("│   ", item.Depth-1))
				}

				if item.Depth > 0 && !item.LastChild {
					fmt.Fprint(w, "├── ")
				}

				if item.Depth > 0 && item.LastChild {
					fmt.Fprint(w, "└── ")
				}

				if item.Depth == 0 {
					fmt.Fprintln(w, output.Hash)
				} else {
					fmt.Fprintln(w, item.Name)
				}

				return nil
			}

			if output.Type == summaryType {
				summary := output.Summary

				fmt.Fprintf(w, "%d directories, %d files present\n",
					summary.NbDir,
					summary.NbFile,
				)

				if summary.SizeTotal > 0 {
					fmt.Fprintf(w, "%s present of %s (%.2f%%)",
						humanize.Bytes(uint64(summary.SizeLocal)),
						humanize.Bytes(uint64(summary.SizeTotal)),
						100.0*float64(summary.SizeLocal)/float64(summary.SizeTotal),
					)
				} else {
					fmt.Fprintf(w, "%s present of unknown", humanize.Bytes(uint64(summary.SizeLocal)))
				}

				fmt.Fprintln(w)
				return nil
			}

			return fmt.Errorf("unknown output type")
		}),
	},
}

type walkState struct {
	ctx     context.Context
	res     cmds.ResponseEmitter
	dagserv dag.DAGService
	node    node.Node

	onlySummary       bool
	depth             int
	name              string
	parentIsDirectory bool
	lastChild         bool
}

func walkBlockStart(ctx context.Context, res cmds.ResponseEmitter, dagserv dag.DAGService, nd node.Node, isDirectory bool, onlySummary bool) (treeSummary, error) {
	return walkBlock(walkState{
		ctx,
		res,
		dagserv,
		nd,
		onlySummary,
		0,
		nd.Cid().String(),
		isDirectory,
		false,
	})
}

// Build a summary of a dag while emiting treeOutput with treeItem to display the graph of files/directories
func walkBlock(state walkState) (treeSummary, error) {

	// Start with the block data size
	result := treeSummary{
		Local:     true,
		SizeLocal: uint64(len(state.node.RawData())),
	}

	isDirectory := false
	// try to decode unixfs
	if pn, ok := state.node.(*dag.ProtoNode); ok {
		if unixfs, err := ft.FromBytes(pn.Data()); err == nil {

			// if cumulative size is available, use it as total size
			cumSize, err := pn.Size()
			if err == nil {
				result.SizeTotal = cumSize
			}

			unixType := unixfs.GetType()

			// To distinguish files from chunk
			isFile := unixType == ft.TFile && state.parentIsDirectory
			isDirectory = unixType == ft.TDirectory

			if isFile {
				result.NbFile++
			}

			// Don't count the root directory if any
			if isDirectory && state.depth != 0 {
				result.NbDir++
			}

			if !state.onlySummary && (isFile || isDirectory) {
				state.res.Emit(&treeOutput{
					Hash: state.node.Cid().String(),
					Type: itemType,
					Item: treeItem{
						Name:      state.name,
						Depth:     state.depth,
						LastChild: state.lastChild,
					},
				})
			}
		}
	}

	nbChild := len(state.node.Links())
	for i, link := range state.node.Links() {
		child, err := state.dagserv.Get(state.ctx, link.Cid)

		isLastChild := i == nbChild-1

		if err == dag.ErrNotFound {
			result.Local = false

			if !state.onlySummary {
				state.res.Emit(&treeOutput{
					Hash: link.Cid.String(),
					Type: itemType,
					Item: treeItem{
						Name:      "[missing]",
						Depth:     state.depth + 1,
						LastChild: isLastChild,
					},
				})
			}

			continue
		}

		if err != nil {
			return result, err
		}

		childSum, err := walkBlock(walkState{
			state.ctx,
			state.res,
			state.dagserv,
			child,
			state.onlySummary,
			state.depth + 1,
			link.Name,
			isDirectory,
			isLastChild,
		})

		if err != nil {
			return treeSummary{}, err
		}

		// aggregate the childs result
		result.Local = result.Local && childSum.Local
		result.SizeLocal += childSum.SizeLocal
		result.NbDir += childSum.NbDir
		result.NbFile += childSum.NbFile
	}

	return result, nil
}

var filesReadCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read a file in a given mfs.",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default,
will read the entire file similar to unix cat.

Examples:

    $ ipfs files read /test/hello
    hello
        `,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to file to be read."),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("offset", "o", "Byte offset to begin reading from."),
		cmdkit.IntOption("count", "n", "Maximum number of bytes to read."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(n.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			res.SetError(fmt.Errorf("%s was not a file.", path), cmdkit.ErrNormal)
			return
		}

		rfd, err := fi.Open(mfs.OpenReadOnly, false)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		defer rfd.Close()

		offset, _, err := req.Option("offset").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if offset < 0 {
			res.SetError(fmt.Errorf("Cannot specify negative offset."), cmdkit.ErrNormal)
			return
		}

		filen, err := rfd.Size()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if int64(offset) > filen {
			res.SetError(fmt.Errorf("Offset was past end of file (%d > %d).", offset, filen), cmdkit.ErrNormal)
			return
		}

		_, err = rfd.Seek(int64(offset), io.SeekStart)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var r io.Reader = &contextReaderWrapper{R: rfd, ctx: req.Context()}
		count, found, err := req.Option("count").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if found {
			if count < 0 {
				res.SetError(fmt.Errorf("Cannot specify negative 'count'."), cmdkit.ErrNormal)
				return
			}
			r = io.LimitReader(r, int64(count))
		}

		res.SetOutput(r)
	},
}

type contextReader interface {
	CtxReadFull(context.Context, []byte) (int, error)
}

type contextReaderWrapper struct {
	R   contextReader
	ctx context.Context
}

func (crw *contextReaderWrapper) Read(b []byte) (int, error) {
	return crw.R.CtxReadFull(crw.ctx, b)
}

var filesMvCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Move files.",
		ShortDescription: `
Move files around. Just like traditional unix mv.

Example:

    $ ipfs files mv /myfs/a/b/c /myfs/foo/newc

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("source", true, false, "Source file to move."),
		cmdkit.StringArg("dest", true, false, "Destination path for file to be moved to."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		src, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		dst, err := checkPath(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = mfs.Mv(n.FilesRoot, src, dst)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}

var filesWriteCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Write to a mutable file in a given filesystem.",
		ShortDescription: `
Write data to a file in a given filesystem. This command allows you to specify
a beginning offset to write to. The entire length of the input will be
written.

If the '--create' option is specified, the file will be created if it does not
exist. Nonexistant intermediate directories will not be created.

Newly created files will have the same CID version and hash function of the
parent directory unless the --cid-version and --hash options are used.

Newly created leaves will be in the legacy format (Protobuf) if the
CID version is 0, or raw is the CID version is non-zero.  Use of the
--raw-leaves option will override this behavior.

If the '--flush' option is set to false, changes will not be propogated to the
merkledag root. This can make operations much faster when doing a large number
of writes to a deeper directory structure.

EXAMPLE:

    echo "hello world" | ipfs files write --create /myfs/a/b/file
    echo "hello world" | ipfs files write --truncate /myfs/a/b/file

WARNING:

Usage of the '--flush=false' option does not guarantee data durability until
the tree has been flushed. This can be accomplished by running 'ipfs files
stat' on the file or any of its ancestors.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to write to."),
		cmdkit.FileArg("data", true, false, "Data to write.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("offset", "o", "Byte offset to begin writing at."),
		cmdkit.BoolOption("create", "e", "Create the file if it does not exist."),
		cmdkit.BoolOption("truncate", "t", "Truncate the file to size zero before writing."),
		cmdkit.IntOption("count", "n", "Maximum number of bytes to read."),
		cmdkit.BoolOption("raw-leaves", "Use raw blocks for newly created leaf nodes. (experimental)"),
		cidVersionOption,
		hashOption,
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		create, _, _ := req.Option("create").Bool()
		trunc, _, _ := req.Option("truncate").Bool()
		flush, _, _ := req.Option("flush").Bool()
		rawLeaves, rawLeavesDef, _ := req.Option("raw-leaves").Bool()

		prefix, err := getPrefix(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		offset, _, err := req.Option("offset").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if offset < 0 {
			res.SetError(fmt.Errorf("cannot have negative write offset"), cmdkit.ErrNormal)
			return
		}

		fi, err := getFileHandle(nd.FilesRoot, path, create, prefix)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if rawLeavesDef {
			fi.RawLeaves = rawLeaves
		}

		wfd, err := fi.Open(mfs.OpenWriteOnly, flush)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		defer func() {
			err := wfd.Close()
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
			}
		}()

		if trunc {
			if err := wfd.Truncate(0); err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		count, countfound, err := req.Option("count").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if countfound && count < 0 {
			res.SetError(fmt.Errorf("cannot have negative byte count"), cmdkit.ErrNormal)
			return
		}

		_, err = wfd.Seek(int64(offset), io.SeekStart)
		if err != nil {
			log.Error("seekfail: ", err)
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var r io.Reader = input
		if countfound {
			r = io.LimitReader(r, int64(count))
		}

		_, err = io.Copy(wfd, r)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}

var filesMkdirCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Make directories.",
		ShortDescription: `
Create the directory if it does not already exist.

The directory will have the same CID version and hash function of the
parent directory unless the --cid-version and --hash options are used.

NOTE: All paths must be absolute.

Examples:

    $ ipfs files mkdir /test/newdir
    $ ipfs files mkdir -p /test/does/not/exist/yet
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to dir to make."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("parents", "p", "No error if existing, make parent directories as needed."),
		cidVersionOption,
		hashOption,
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dashp, _, _ := req.Option("parents").Bool()
		dirtomake, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		flush, _, _ := req.Option("flush").Bool()

		prefix, err := getPrefix(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		root := n.FilesRoot

		err = mfs.Mkdir(root, dirtomake, mfs.MkdirOpts{
			Mkparents: dashp,
			Flush:     flush,
			Prefix:    prefix,
		})
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}

var filesFlushCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Flush a given path's data to disk.",
		ShortDescription: `
Flush a given path to disk. This is only useful when other commands
are run with the '--flush=false'.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", false, false, "Path to flush. Default: '/'."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path := "/"
		if len(req.Arguments()) > 0 {
			path = req.Arguments()[0]
		}

		err = mfs.FlushPath(nd.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}

var filesChcidCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Change the cid version or hash function of the root node of a given path.",
		ShortDescription: `
Change the cid version or hash function of the root node of a given path.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", false, false, "Path to change. Default: '/'."),
	},
	Options: []cmdkit.Option{
		cidVersionOption,
		hashOption,
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path := "/"
		if len(req.Arguments()) > 0 {
			path = req.Arguments()[0]
		}

		flush, _, _ := req.Option("flush").Bool()

		prefix, err := getPrefix(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = updatePath(nd.FilesRoot, path, prefix, flush)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}

func updatePath(rt *mfs.Root, pth string, prefix *cid.Prefix, flush bool) error {
	if prefix == nil {
		return nil
	}

	nd, err := mfs.Lookup(rt, pth)
	if err != nil {
		return err
	}

	switch n := nd.(type) {
	case *mfs.Directory:
		n.SetPrefix(prefix)
	default:
		return fmt.Errorf("can only update directories")
	}

	if flush {
		nd.Flush()
	}

	return nil
}

var filesRmCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove a file.",
		ShortDescription: `
Remove files or directories.

    $ ipfs files rm /foo
    $ ipfs files ls /bar
    cat
    dog
    fish
    $ ipfs files rm -r /bar
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, true, "File to remove."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("recursive", "r", "Recursively remove directories."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		defer res.SetOutput(nil)

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if path == "/" {
			res.SetError(fmt.Errorf("cannot delete root"), cmdkit.ErrNormal)
			return
		}

		// 'rm a/b/c/' will fail unless we trim the slash at the end
		if path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}

		dir, name := gopath.Split(path)
		parent, err := mfs.Lookup(nd.FilesRoot, dir)
		if err != nil {
			res.SetError(fmt.Errorf("parent lookup: %s", err), cmdkit.ErrNormal)
			return
		}

		pdir, ok := parent.(*mfs.Directory)
		if !ok {
			res.SetError(fmt.Errorf("No such file or directory: %s", path), cmdkit.ErrNormal)
			return
		}

		dashr, _, _ := req.Option("r").Bool()

		var success bool
		defer func() {
			if success {
				err := pdir.Flush()
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
			}
		}()

		// if '-r' specified, don't check file type (in bad scenarios, the block may not exist)
		if dashr {
			err := pdir.Unlink(name)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			success = true
			return
		}

		childi, err := pdir.Child(name)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		switch childi.(type) {
		case *mfs.Directory:
			res.SetError(fmt.Errorf("%s is a directory, use -r to remove directories", path), cmdkit.ErrNormal)
			return
		default:
			err := pdir.Unlink(name)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			success = true
		}
	},
}

func getPrefix(req oldcmds.Request) (*cid.Prefix, error) {
	cidVer, cidVerSet, _ := req.Option("cid-version").Int()
	hashFunStr, hashFunSet, _ := req.Option("hash").String()

	if !cidVerSet && !hashFunSet {
		return nil, nil
	}

	if hashFunSet && cidVer == 0 {
		cidVer = 1
	}

	prefix, err := dag.PrefixForCidVersion(cidVer)
	if err != nil {
		return nil, err
	}

	if hashFunSet {
		hashFunCode, ok := mh.Names[strings.ToLower(hashFunStr)]
		if !ok {
			return nil, fmt.Errorf("unrecognized hash function: %s", strings.ToLower(hashFunStr))
		}
		prefix.MhType = hashFunCode
		prefix.MhLength = -1
	}

	return &prefix, nil
}

func getFileHandle(r *mfs.Root, path string, create bool, prefix *cid.Prefix) (*mfs.File, error) {
	target, err := mfs.Lookup(r, path)
	switch err {
	case nil:
		fi, ok := target.(*mfs.File)
		if !ok {
			return nil, fmt.Errorf("%s was not a file", path)
		}
		return fi, nil

	case os.ErrNotExist:
		if !create {
			return nil, err
		}

		// if create is specified and the file doesnt exist, we create the file
		dirname, fname := gopath.Split(path)
		pdiri, err := mfs.Lookup(r, dirname)
		if err != nil {
			log.Error("lookupfail ", dirname)
			return nil, err
		}
		pdir, ok := pdiri.(*mfs.Directory)
		if !ok {
			return nil, fmt.Errorf("%s was not a directory", dirname)
		}
		if prefix == nil {
			prefix = pdir.GetPrefix()
		}

		nd := dag.NodeWithData(ft.FilePBData(nil, 0))
		nd.SetPrefix(prefix)
		err = pdir.AddChild(fname, nd)
		if err != nil {
			return nil, err
		}

		fsn, err := pdir.Child(fname)
		if err != nil {
			return nil, err
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			return nil, errors.New("Expected *mfs.File, didnt get it. This is likely a race condition.")
		}
		return fi, nil

	default:
		return nil, err
	}
}

func checkPath(p string) (string, error) {
	if len(p) == 0 {
		return "", fmt.Errorf("Paths must not be empty.")
	}

	if p[0] != '/' {
		return "", fmt.Errorf("Paths must start with a leading slash.")
	}

	cleaned := gopath.Clean(p)
	if p[len(p)-1] == '/' && p != "/" {
		cleaned += "/"
	}
	return cleaned, nil
}

// copy+pasted from ../commands.go
func unwrapOutput(i interface{}) (interface{}, error) {
	var (
		ch <-chan interface{}
		ok bool
	)

	if ch, ok = i.(<-chan interface{}); !ok {
		return nil, e.TypeErr(ch, i)
	}

	return <-ch, nil
}
