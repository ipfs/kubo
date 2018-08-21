package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"sort"
	"strings"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	bservice "gx/ipfs/QmTZZrpd9o4vpYr9TEADW2EoJ9fzUtAgpXqjxZHbKR2T15/go-blockservice"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"
	resolver "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path/resolver"
	ft "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs"
	uio "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/io"

	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	mfs "gx/ipfs/QmQeLRo7dHKpmREWkAUXRAri4Bro3BqrDVJJHjHHmKSHMc/go-mfs"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	cmds "gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"
	offline "gx/ipfs/QmVozMmsgK2PYyaHQsrcWLBYigb1m6mW8YhCBG2Cb4Uxq9/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

var flog = logging.Logger("cmds/files")

// FilesCmd is the 'ipfs files' command
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
		"read":  lgc.NewCommand(filesReadCmd),
		"write": filesWriteCmd,
		"mv":    lgc.NewCommand(filesMvCmd),
		"cp":    lgc.NewCommand(filesCpCmd),
		"ls":    lgc.NewCommand(filesLsCmd),
		"mkdir": lgc.NewCommand(filesMkdirCmd),
		"stat":  filesStatCmd,
		"rm":    lgc.NewCommand(filesRmCmd),
		"flush": lgc.NewCommand(filesFlushCmd),
		"chcid": lgc.NewCommand(filesChcidCmd),
	},
}

var cidVersionOption = cmdkit.IntOption("cid-version", "cid-ver", "Cid version to use. (experimental)")
var hashOption = cmdkit.StringOption("hash", "Hash function to use. Will set Cid version to 1 if used. (experimental)")

var errFormat = errors.New("format was set by multiple options. Only one format option is allowed")

type statOutput struct {
	Hash           string
	Size           uint64
	CumulativeSize uint64
	Blocks         int
	Type           string
	WithLocality   bool   `json:",omitempty"`
	Local          bool   `json:",omitempty"`
	SizeLocal      uint64 `json:",omitempty"`
}

const defaultStatFormat = `<hash>
Size: <size>
CumulativeSize: <cumulsize>
ChildBlocks: <childs>
Type: <type>`

var filesStatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Display file status.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("path", true, false, "Path to node to stat."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "Print statistics in given format. Allowed tokens: "+
			"<hash> <size> <cumulsize> <type> <childs>. Conflicts with other format options.").WithDefault(defaultStatFormat),
		cmdkit.BoolOption("hash", "Print only hash. Implies '--format=<hash>'. Conflicts with other format options."),
		cmdkit.BoolOption("size", "Print only size. Implies '--format=<cumulsize>'. Conflicts with other format options."),
		cmdkit.BoolOption("with-local", "Compute the amount of the dag that is local, and if possible the total size"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {

		_, err := statGetFormatOptions(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrClient)
		}

		node, err := GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		withLocal, _ := req.Options["with-local"].(bool)

		var dagserv ipld.DAGService
		if withLocal {
			// an offline DAGService will not fetch from the network
			dagserv = dag.NewDAGService(bservice.New(
				node.Blockstore,
				offline.Exchange(node.Blockstore),
			))
		} else {
			dagserv = node.DAG
		}

		nd, err := getNodeFromPath(req.Context, node, dagserv, path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		o, err := statNode(nd)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !withLocal {
			cmds.EmitOnce(res, o)
			return
		}

		local, sizeLocal, err := walkBlock(req.Context, dagserv, nd)

		o.WithLocality = true
		o.Local = local
		o.SizeLocal = sizeLocal

		cmds.EmitOnce(res, o)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			out, ok := v.(*statOutput)
			if !ok {
				return e.TypeErr(out, v)
			}

			s, _ := statGetFormatOptions(req)
			s = strings.Replace(s, "<hash>", out.Hash, -1)
			s = strings.Replace(s, "<size>", fmt.Sprintf("%d", out.Size), -1)
			s = strings.Replace(s, "<cumulsize>", fmt.Sprintf("%d", out.CumulativeSize), -1)
			s = strings.Replace(s, "<childs>", fmt.Sprintf("%d", out.Blocks), -1)
			s = strings.Replace(s, "<type>", out.Type, -1)

			fmt.Fprintln(w, s)

			if out.WithLocality {
				fmt.Fprintf(w, "Local: %s of %s (%.2f%%)\n",
					humanize.Bytes(out.SizeLocal),
					humanize.Bytes(out.CumulativeSize),
					100.0*float64(out.SizeLocal)/float64(out.CumulativeSize),
				)
			}

			return nil
		}),
	},
	Type: statOutput{},
}

func moreThanOne(a, b, c bool) bool {
	return a && b || b && c || a && c
}

func statGetFormatOptions(req *cmds.Request) (string, error) {

	hash, _ := req.Options["hash"].(bool)
	size, _ := req.Options["size"].(bool)
	format, _ := req.Options["format"].(string)

	if moreThanOne(hash, size, format != defaultStatFormat) {
		return "", errFormat
	}

	if hash {
		return "<hash>", nil
	} else if size {
		return "<cumulsize>", nil
	} else {
		return format, nil
	}
}

func statNode(nd ipld.Node) (*statOutput, error) {
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
		switch d.GetType() {
		case ft.TDirectory, ft.THAMTShard:
			ndtype = "directory"
		case ft.TFile, ft.TMetadata, ft.TRaw:
			ndtype = "file"
		default:
			return nil, fmt.Errorf("unrecognized node type: %s", d.GetType())
		}

		return &statOutput{
			Hash:           c.String(),
			Blocks:         len(nd.Links()),
			Size:           d.GetFilesize(),
			CumulativeSize: cumulsize,
			Type:           ndtype,
		}, nil
	case *dag.RawNode:
		return &statOutput{
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

func walkBlock(ctx context.Context, dagserv ipld.DAGService, nd ipld.Node) (bool, uint64, error) {
	// Start with the block data size
	sizeLocal := uint64(len(nd.RawData()))

	local := true

	for _, link := range nd.Links() {
		child, err := dagserv.Get(ctx, link.Cid)

		if err == ipld.ErrNotFound {
			local = false
			continue
		}

		if err != nil {
			return local, sizeLocal, err
		}

		childLocal, childLocalSize, err := walkBlock(ctx, dagserv, child)

		if err != nil {
			return local, sizeLocal, err
		}

		// Recursively add the child size
		local = local && childLocal
		sizeLocal += childLocalSize
	}

	return local, sizeLocal, nil
}

var filesCpCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Copy files into mfs.",
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
			res.SetError(fmt.Errorf("cp: cannot get node from path %s: %s", src, err), cmdkit.ErrNormal)
			return
		}

		err = mfs.PutNode(node.FilesRoot, dst, nd)
		if err != nil {
			res.SetError(fmt.Errorf("cp: cannot put node in path %s: %s", dst, err), cmdkit.ErrNormal)
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

func getNodeFromPath(ctx context.Context, node *core.IpfsNode, dagservice ipld.DAGService, p string) (ipld.Node, error) {
	switch {
	case strings.HasPrefix(p, "/ipfs/"):
		np, err := path.ParsePath(p)
		if err != nil {
			return nil, err
		}

		resolver := &resolver.Resolver{
			DAG:         dagservice,
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
		cmdkit.BoolOption("U", "Do not sort; list entries in directory order."),
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
			out := &filesLsOutput{[]mfs.NodeListing{mfs.NodeListing{Name: name}}}
			if long {
				out.Entries[0].Type = int(fsn.Type())

				size, err := fsn.Size()
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
				out.Entries[0].Size = size

				nd, err := fsn.GetNode()
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
				out.Entries[0].Hash = nd.Cid().String()
			}
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

			noSort, _, _ := res.Request().Option("U").Bool()
			if !noSort {
				sort.Slice(out.Entries, func(i, j int) bool {
					return strings.Compare(out.Entries[i].Name, out.Entries[j].Name) < 0
				})
			}

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
			res.SetError(fmt.Errorf("%s was not a file", path), cmdkit.ErrNormal)
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
			res.SetError(fmt.Errorf("cannot specify negative offset"), cmdkit.ErrNormal)
			return
		}

		filen, err := rfd.Size()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if int64(offset) > filen {
			res.SetError(fmt.Errorf("offset was past end of file (%d > %d)", offset, filen), cmdkit.ErrNormal)
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
				res.SetError(fmt.Errorf("cannot specify negative 'count'"), cmdkit.ErrNormal)
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

var filesWriteCmd = &cmds.Command{
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		path, err := checkPath(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		create, _ := req.Options["create"].(bool)
		trunc, _ := req.Options["truncate"].(bool)
		flush, _ := req.Options["flush"].(bool)
		rawLeaves, rawLeavesDef := req.Options["raw-leaves"].(bool)

		prefix, err := getPrefixNew(req)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		nd, err := GetNode(env)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		offset, _ := req.Options["offset"].(int)
		if offset < 0 {
			re.SetError(fmt.Errorf("cannot have negative write offset"), cmdkit.ErrNormal)
			return
		}

		fi, err := getFileHandle(nd.FilesRoot, path, create, prefix)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		if rawLeavesDef {
			fi.RawLeaves = rawLeaves
		}

		wfd, err := fi.Open(mfs.OpenWriteOnly, flush)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		defer func() {
			err := wfd.Close()
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
			}
		}()

		if trunc {
			if err := wfd.Truncate(0); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		count, countfound := req.Options["count"].(int)
		if countfound && count < 0 {
			re.SetError(fmt.Errorf("cannot have negative byte count"), cmdkit.ErrNormal)
			return
		}

		_, err = wfd.Seek(int64(offset), io.SeekStart)
		if err != nil {
			flog.Error("seekfail: ", err)
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		input, err := req.Files.NextFile()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var r io.Reader = input
		if countfound {
			r = io.LimitReader(r, int64(count))
		}

		_, err = io.Copy(wfd, r)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
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
			Mkparents:  dashp,
			Flush:      flush,
			CidBuilder: prefix,
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

func updatePath(rt *mfs.Root, pth string, builder cid.Builder, flush bool) error {
	if builder == nil {
		return nil
	}

	nd, err := mfs.Lookup(rt, pth)
	if err != nil {
		return err
	}

	switch n := nd.(type) {
	case *mfs.Directory:
		n.SetCidBuilder(builder)
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
			res.SetError(fmt.Errorf("no such file or directory: %s", path), cmdkit.ErrNormal)
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

func getPrefixNew(req *cmds.Request) (cid.Builder, error) {
	cidVer, cidVerSet := req.Options["cid-version"].(int)
	hashFunStr, hashFunSet := req.Options["hash"].(string)

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

func getPrefix(req oldcmds.Request) (cid.Builder, error) {
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

func getFileHandle(r *mfs.Root, path string, create bool, builder cid.Builder) (*mfs.File, error) {
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
			flog.Error("lookupfail ", dirname)
			return nil, err
		}
		pdir, ok := pdiri.(*mfs.Directory)
		if !ok {
			return nil, fmt.Errorf("%s was not a directory", dirname)
		}
		if builder == nil {
			builder = pdir.GetCidBuilder()
		}

		nd := dag.NodeWithData(ft.FilePBData(nil, 0))
		nd.SetCidBuilder(builder)
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
			return nil, errors.New("expected *mfs.File, didnt get it. This is likely a race condition")
		}
		return fi, nil

	default:
		return nil, err
	}
}

func checkPath(p string) (string, error) {
	if len(p) == 0 {
		return "", fmt.Errorf("paths must not be empty")
	}

	if p[0] != '/' {
		return "", fmt.Errorf("paths must start with a leading slash")
	}

	cleaned := gopath.Clean(p)
	if p[len(p)-1] == '/' && p != "/" {
		cleaned += "/"
	}
	return cleaned, nil
}
