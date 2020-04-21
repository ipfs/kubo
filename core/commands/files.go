package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"sort"
	"strings"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"

	"github.com/dustin/go-humanize"
	bservice "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	ft "github.com/ipfs/go-unixfs"
	iface "github.com/ipfs/interface-go-ipfs-core"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	mh "github.com/multiformats/go-multihash"
)

var flog = logging.Logger("cmds/files")

// FilesCmd is the 'ipfs files' command
var FilesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with unixfs files.",
		ShortDescription: `
Files is an API for manipulating IPFS objects as if they were a Unix
filesystem.

The files facility interacts with MFS (Mutable File System). MFS acts as a
single, dynamic filesystem mount. MFS has a root CID that is transparently
updated when a change happens (and can be checked with "ipfs files stat /").

All files and folders within MFS are respected and will not be cleaned up
during garbage collections. MFS is independent from the list of pinned items
("ipfs pin ls"). Calls to "ipfs pin add" and "ipfs pin rm" will add and remove
pins independently of MFS. If MFS content that was
additionally pinned is removed by calling "ipfs files rm", it will still
remain pinned.

Content added with "ipfs add" (which by default also becomes pinned), is not
added to MFS. Any content can be put into MFS with the command "ipfs files cp
/ipfs/<cid> /some/path/".


NOTE:
Most of the subcommands of 'ipfs files' accept the '--flush' flag. It defaults
to true. Use caution when setting this flag to false. It will improve
performance for large numbers of file operations, but it does so at the cost
of consistency guarantees. If the daemon is unexpectedly killed before running
'ipfs files flush' on the files in question, then data may be lost. This also
applies to run 'ipfs repo gc' concurrently with '--flush=false'
operations.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(filesFlushOptionName, "f", "Flush target and ancestors after write.").WithDefault(true),
	},
	Subcommands: map[string]*cmds.Command{
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

const (
	filesCidVersionOptionName = "cid-version"
	filesHashOptionName       = "hash"
)

var cidVersionOption = cmds.IntOption(filesCidVersionOptionName, "cid-ver", "Cid version to use. (experimental)")
var hashOption = cmds.StringOption(filesHashOptionName, "Hash function to use. Will set Cid version to 1 if used. (experimental)")

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

const (
	defaultStatFormat = `<hash>
Size: <size>
CumulativeSize: <cumulsize>
ChildBlocks: <childs>
Type: <type>`
	filesFormatOptionName    = "format"
	filesSizeOptionName      = "size"
	filesWithLocalOptionName = "with-local"
)

var filesStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Display file status.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to node to stat."),
	},
	Options: []cmds.Option{
		cmds.StringOption(filesFormatOptionName, "Print statistics in given format. Allowed tokens: "+
			"<hash> <size> <cumulsize> <type> <childs>. Conflicts with other format options.").WithDefault(defaultStatFormat),
		cmds.BoolOption(filesHashOptionName, "Print only hash. Implies '--format=<hash>'. Conflicts with other format options."),
		cmds.BoolOption(filesSizeOptionName, "Print only size. Implies '--format=<cumulsize>'. Conflicts with other format options."),
		cmds.BoolOption(filesWithLocalOptionName, "Compute the amount of the dag that is local, and if possible the total size"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		_, err := statGetFormatOptions(req)
		if err != nil {
			return cmds.Errorf(cmds.ErrClient, err.Error())
		}

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		path, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}

		withLocal, _ := req.Options[filesWithLocalOptionName].(bool)

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

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

		nd, err := getNodeFromPath(req.Context, node, api, path)
		if err != nil {
			return err
		}

		o, err := statNode(nd, enc)
		if err != nil {
			return err
		}

		if !withLocal {
			return cmds.EmitOnce(res, o)
		}

		local, sizeLocal, err := walkBlock(req.Context, dagserv, nd)
		if err != nil {
			return err
		}

		o.WithLocality = true
		o.Local = local
		o.SizeLocal = sizeLocal

		return cmds.EmitOnce(res, o)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *statOutput) error {
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

	hash, _ := req.Options[filesHashOptionName].(bool)
	size, _ := req.Options[filesSizeOptionName].(bool)
	format, _ := req.Options[filesFormatOptionName].(string)

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

func statNode(nd ipld.Node, enc cidenc.Encoder) (*statOutput, error) {
	c := nd.Cid()

	cumulsize, err := nd.Size()
	if err != nil {
		return nil, err
	}

	switch n := nd.(type) {
	case *dag.ProtoNode:
		d, err := ft.FSNodeFromBytes(n.Data())
		if err != nil {
			return nil, err
		}

		var ndtype string
		switch d.Type() {
		case ft.TDirectory, ft.THAMTShard:
			ndtype = "directory"
		case ft.TFile, ft.TMetadata, ft.TRaw:
			ndtype = "file"
		default:
			return nil, fmt.Errorf("unrecognized node type: %s", d.Type())
		}

		return &statOutput{
			Hash:           enc.Encode(c),
			Blocks:         len(nd.Links()),
			Size:           d.FileSize(),
			CumulativeSize: cumulsize,
			Type:           ndtype,
		}, nil
	case *dag.RawNode:
		return &statOutput{
			Hash:           enc.Encode(c),
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

var filesCpCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Copy any IPFS files and directories into MFS (or copy within MFS).",
		ShortDescription: `
"ipfs files cp" can be used to copy any IPFS file or directory (usually in the
form /ipfs/<CID>, but also any resolvable path), into the Mutable File System
(MFS).

It can also be used to copy files within MFS, but in the case when an
IPFS-path matches an existing MFS path, the IPFS path wins.

In order to add content to MFS from disk, you can use "ipfs add" to obtain the
IPFS Content Identifier and then "ipfs files cp" to copy it into MFS:

$ ipfs add --quieter --pin=false <your file>
# ...
# ... outputs the root CID at the end
$ ipfs cp /ipfs/<CID> /your/desired/mfs/path
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source IPFS or MFS path to copy."),
		cmds.StringArg("dest", true, false, "Destination within MFS."),
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

		flush, _ := req.Options[filesFlushOptionName].(bool)

		src, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}
		src = strings.TrimRight(src, "/")

		dst, err := checkPath(req.Arguments[1])
		if err != nil {
			return err
		}

		if dst[len(dst)-1] == '/' {
			dst += gopath.Base(src)
		}

		node, err := getNodeFromPath(req.Context, nd, api, src)
		if err != nil {
			return fmt.Errorf("cp: cannot get node from path %s: %s", src, err)
		}

		err = mfs.PutNode(nd.FilesRoot, dst, node)
		if err != nil {
			return fmt.Errorf("cp: cannot put node in path %s: %s", dst, err)
		}

		if flush {
			_, err := mfs.FlushPath(req.Context, nd.FilesRoot, dst)
			if err != nil {
				return fmt.Errorf("cp: cannot flush the created file %s: %s", dst, err)
			}
		}

		return nil
	},
}

func getNodeFromPath(ctx context.Context, node *core.IpfsNode, api iface.CoreAPI, p string) (ipld.Node, error) {
	switch {
	case strings.HasPrefix(p, "/ipfs/"):
		return api.ResolveNode(ctx, path.New(p))
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

const (
	longOptionName     = "long"
	dontSortOptionName = "U"
)

var filesLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to show listing for. Defaults to '/'."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(longOptionName, "l", "Use long listing format."),
		cmds.BoolOption(dontSortOptionName, "Do not sort; list entries in directory order."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var arg string

		if len(req.Arguments) == 0 {
			arg = "/"
		} else {
			arg = req.Arguments[0]
		}

		path, err := checkPath(arg)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		fsn, err := mfs.Lookup(nd.FilesRoot, path)
		if err != nil {
			return err
		}

		long, _ := req.Options[longOptionName].(bool)

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		switch fsn := fsn.(type) {
		case *mfs.Directory:
			if !long {
				var output []mfs.NodeListing
				names, err := fsn.ListNames(req.Context)
				if err != nil {
					return err
				}

				for _, name := range names {
					output = append(output, mfs.NodeListing{
						Name: name,
					})
				}
				return cmds.EmitOnce(res, &filesLsOutput{output})
			}
			listing, err := fsn.List(req.Context)
			if err != nil {
				return err
			}
			return cmds.EmitOnce(res, &filesLsOutput{listing})
		case *mfs.File:
			_, name := gopath.Split(path)
			out := &filesLsOutput{[]mfs.NodeListing{{Name: name}}}
			if long {
				out.Entries[0].Type = int(fsn.Type())

				size, err := fsn.Size()
				if err != nil {
					return err
				}
				out.Entries[0].Size = size

				nd, err := fsn.GetNode()
				if err != nil {
					return err
				}
				out.Entries[0].Hash = enc.Encode(nd.Cid())
			}
			return cmds.EmitOnce(res, out)
		default:
			return errors.New("unrecognized type")
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *filesLsOutput) error {
			noSort, _ := req.Options[dontSortOptionName].(bool)
			if !noSort {
				sort.Slice(out.Entries, func(i, j int) bool {
					return strings.Compare(out.Entries[i].Name, out.Entries[j].Name) < 0
				})
			}

			long, _ := req.Options[longOptionName].(bool)
			for _, o := range out.Entries {
				if long {
					if o.Type == int(mfs.TDir) {
						o.Name += "/"
					}
					fmt.Fprintf(w, "%s\t%s\t%d\n", o.Name, o.Hash, o.Size)
				} else {
					fmt.Fprintf(w, "%s\n", o.Name)
				}
			}

			return nil
		}),
	},
	Type: filesLsOutput{},
}

const (
	filesOffsetOptionName = "offset"
	filesCountOptionName  = "count"
)

var filesReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read a file in a given MFS.",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default,
it will read the entire file similar to the Unix cat.

Examples:

    $ ipfs files read /test/hello
    hello
        `,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to file to be read."),
	},
	Options: []cmds.Option{
		cmds.Int64Option(filesOffsetOptionName, "o", "Byte offset to begin reading from."),
		cmds.Int64Option(filesCountOptionName, "n", "Maximum number of bytes to read."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		path, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}

		fsn, err := mfs.Lookup(nd.FilesRoot, path)
		if err != nil {
			return err
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			return fmt.Errorf("%s was not a file", path)
		}

		rfd, err := fi.Open(mfs.Flags{Read: true})
		if err != nil {
			return err
		}

		defer rfd.Close()

		offset, _ := req.Options[offsetOptionName].(int64)
		if offset < 0 {
			return fmt.Errorf("cannot specify negative offset")
		}

		filen, err := rfd.Size()
		if err != nil {
			return err
		}

		if int64(offset) > filen {
			return fmt.Errorf("offset was past end of file (%d > %d)", offset, filen)
		}

		_, err = rfd.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return err
		}

		var r io.Reader = &contextReaderWrapper{R: rfd, ctx: req.Context}
		count, found := req.Options[filesCountOptionName].(int64)
		if found {
			if count < 0 {
				return fmt.Errorf("cannot specify negative 'count'")
			}
			r = io.LimitReader(r, int64(count))
		}
		return res.Emit(r)
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

var filesMvCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move files.",
		ShortDescription: `
Move files around. Just like the traditional Unix mv.

Example:

    $ ipfs files mv /myfs/a/b/c /myfs/foo/newc

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source file to move."),
		cmds.StringArg("dest", true, false, "Destination path for file to be moved to."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)

		src, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}
		dst, err := checkPath(req.Arguments[1])
		if err != nil {
			return err
		}

		err = mfs.Mv(nd.FilesRoot, src, dst)
		if err == nil && flush {
			_, err = mfs.FlushPath(req.Context, nd.FilesRoot, "/")
		}
		return err
	},
}

const (
	filesCreateOptionName    = "create"
	filesParentsOptionName   = "parents"
	filesTruncateOptionName  = "truncate"
	filesRawLeavesOptionName = "raw-leaves"
	filesFlushOptionName     = "flush"
)

var filesWriteCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write to a mutable file in a given filesystem.",
		ShortDescription: `
Write data to a file in a given filesystem. This command allows you to specify
a beginning offset to write to. The entire length of the input will be
written.

If the '--create' option is specified, the file will be created if it does not
exist. Nonexistent intermediate directories will not be created unless the
'--parents' option is specified.

Newly created files will have the same CID version and hash function of the
parent directory unless the '--cid-version' and '--hash' options are used.

Newly created leaves will be in the legacy format (Protobuf) if the
CID version is 0, or raw if the CID version is non-zero.  Use of the
'--raw-leaves' option will override this behavior.

If the '--flush' option is set to false, changes will not be propagated to the
merkledag root. This can make operations much faster when doing a large number
of writes to a deeper directory structure.

EXAMPLE:

    echo "hello world" | ipfs files write --create --parents /myfs/a/b/file
    echo "hello world" | ipfs files write --truncate /myfs/a/b/file

WARNING:

Usage of the '--flush=false' option does not guarantee data durability until
the tree has been flushed. This can be accomplished by running 'ipfs files
stat' on the file or any of its ancestors.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to write to."),
		cmds.FileArg("data", true, false, "Data to write.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.Int64Option(filesOffsetOptionName, "o", "Byte offset to begin writing at."),
		cmds.BoolOption(filesCreateOptionName, "e", "Create the file if it does not exist."),
		cmds.BoolOption(filesParentsOptionName, "p", "Make parent directories as needed."),
		cmds.BoolOption(filesTruncateOptionName, "t", "Truncate the file to size zero before writing."),
		cmds.Int64Option(filesCountOptionName, "n", "Maximum number of bytes to read."),
		cmds.BoolOption(filesRawLeavesOptionName, "Use raw blocks for newly created leaf nodes. (experimental)"),
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) (retErr error) {
		path, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}

		create, _ := req.Options[filesCreateOptionName].(bool)
		mkParents, _ := req.Options[filesParentsOptionName].(bool)
		trunc, _ := req.Options[filesTruncateOptionName].(bool)
		flush, _ := req.Options[filesFlushOptionName].(bool)
		rawLeaves, rawLeavesDef := req.Options[filesRawLeavesOptionName].(bool)

		prefix, err := getPrefixNew(req)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		offset, _ := req.Options[filesOffsetOptionName].(int64)
		if offset < 0 {
			return fmt.Errorf("cannot have negative write offset")
		}

		if mkParents {
			err := ensureContainingDirectoryExists(nd.FilesRoot, path, prefix)
			if err != nil {
				return err
			}
		}

		fi, err := getFileHandle(nd.FilesRoot, path, create, prefix)
		if err != nil {
			return err
		}
		if rawLeavesDef {
			fi.RawLeaves = rawLeaves
		}

		wfd, err := fi.Open(mfs.Flags{Write: true, Sync: flush})
		if err != nil {
			return err
		}

		defer func() {
			err := wfd.Close()
			if err != nil {
				if retErr == nil {
					retErr = err
				} else {
					flog.Error("files: error closing file mfs file descriptor", err)
				}
			}
		}()

		if trunc {
			if err := wfd.Truncate(0); err != nil {
				return err
			}
		}

		count, countfound := req.Options[filesCountOptionName].(int64)
		if countfound && count < 0 {
			return fmt.Errorf("cannot have negative byte count")
		}

		_, err = wfd.Seek(int64(offset), io.SeekStart)
		if err != nil {
			flog.Error("seekfail: ", err)
			return err
		}

		var r io.Reader
		r, err = cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		if countfound {
			r = io.LimitReader(r, int64(count))
		}

		_, err = io.Copy(wfd, r)
		return err
	},
}

var filesMkdirCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to dir to make."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(filesParentsOptionName, "p", "No error if existing, make parent directories as needed."),
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		dashp, _ := req.Options[filesParentsOptionName].(bool)
		dirtomake, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)

		prefix, err := getPrefix(req)
		if err != nil {
			return err
		}
		root := n.FilesRoot

		err = mfs.Mkdir(root, dirtomake, mfs.MkdirOpts{
			Mkparents:  dashp,
			Flush:      flush,
			CidBuilder: prefix,
		})

		return err
	},
}

type flushRes struct {
	Cid string
}

var filesFlushCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Flush a given path's data to disk.",
		ShortDescription: `
Flush a given path to the disk. This is only useful when other commands
are run with the '--flush=false'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to flush. Default: '/'."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		path := "/"
		if len(req.Arguments) > 0 {
			path = req.Arguments[0]
		}

		n, err := mfs.FlushPath(req.Context, nd.FilesRoot, path)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &flushRes{enc.Encode(n.Cid())})
	},
	Type: flushRes{},
}

var filesChcidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change the cid version or hash function of the root node of a given path.",
		ShortDescription: `
Change the cid version or hash function of the root node of a given path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to change. Default: '/'."),
	},
	Options: []cmds.Option{
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		path := "/"
		if len(req.Arguments) > 0 {
			path = req.Arguments[0]
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)

		prefix, err := getPrefix(req)
		if err != nil {
			return err
		}

		err = updatePath(nd.FilesRoot, path, prefix)
		if err == nil && flush {
			_, err = mfs.FlushPath(req.Context, nd.FilesRoot, path)
		}
		return err
	},
}

func updatePath(rt *mfs.Root, pth string, builder cid.Builder) error {
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

	return nil
}

var filesRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, true, "File to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(recursiveOptionName, "r", "Recursively remove directories."),
		cmds.BoolOption(forceOptionName, "Forcibly remove target at path; implies -r for directories"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		path, err := checkPath(req.Arguments[0])
		if err != nil {
			return err
		}

		if path == "/" {
			return fmt.Errorf("cannot delete root")
		}

		// 'rm a/b/c/' will fail unless we trim the slash at the end
		if path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}

		// if '--force' specified, it will remove anything else,
		// including file, directory, corrupted node, etc
		force, _ := req.Options[forceOptionName].(bool)

		dir, name := gopath.Split(path)

		pdir, err := getParentDir(nd.FilesRoot, dir)
		if err != nil {
			if force && err == os.ErrNotExist {
				return nil
			}
			return fmt.Errorf("parent lookup: %s", err)
		}

		if force {
			err := pdir.Unlink(name)
			if err != nil {
				if err == os.ErrNotExist {
					return nil
				}
				return err
			}
			return pdir.Flush()
		}

		// get child node by name, when the node is corrupted and nonexistent,
		// it will return specific error.
		child, err := pdir.Child(name)
		if err != nil {
			return err
		}

		dashr, _ := req.Options[recursiveOptionName].(bool)

		switch child.(type) {
		case *mfs.Directory:
			if !dashr {
				return fmt.Errorf("%s is a directory, use -r to remove directories", path)
			}
		}

		err = pdir.Unlink(name)
		if err != nil {
			return err
		}

		return pdir.Flush()
	},
}

func getPrefixNew(req *cmds.Request) (cid.Builder, error) {
	cidVer, cidVerSet := req.Options[filesCidVersionOptionName].(int)
	hashFunStr, hashFunSet := req.Options[filesHashOptionName].(string)

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

func getPrefix(req *cmds.Request) (cid.Builder, error) {
	cidVer, cidVerSet := req.Options[filesCidVersionOptionName].(int)
	hashFunStr, hashFunSet := req.Options[filesHashOptionName].(string)

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

func ensureContainingDirectoryExists(r *mfs.Root, path string, builder cid.Builder) error {
	dirtomake := gopath.Dir(path)

	if dirtomake == "/" {
		return nil
	}

	return mfs.Mkdir(r, dirtomake, mfs.MkdirOpts{
		Mkparents:  true,
		CidBuilder: builder,
	})
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

		// if create is specified and the file doesn't exist, we create the file
		dirname, fname := gopath.Split(path)
		pdir, err := getParentDir(r, dirname)
		if err != nil {
			return nil, err
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
			return nil, errors.New("expected *mfs.File, didn't get it. This is likely a race condition")
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

func getParentDir(root *mfs.Root, dir string) (*mfs.Directory, error) {
	parent, err := mfs.Lookup(root, dir)
	if err != nil {
		return nil, err
	}

	pdir, ok := parent.(*mfs.Directory)
	if !ok {
		return nil, errors.New("expected *mfs.Directory, didn't get it. This is likely a race condition")
	}
	return pdir, nil
}
