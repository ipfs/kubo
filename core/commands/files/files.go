package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mfs "github.com/ipfs/go-ipfs/mfs"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("cmds/files")

var FilesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate unixfs files.",
		ShortDescription: `
Files is an API for manipulating ipfs objects as if they were a unix filesystem.

Note:
Most of the subcommands of 'ipfs files' accept the '--flush' flag. Use caution
when using this flag, It will improve performance for large numbers of file
operations, but it does so at the cost of consistency guarantees. If the daemon
is unexpectedly killed before running 'ipfs files flush' on the files in question,
then data may be lost. This also applies to running 'ipfs repo gc' concurrently
with '--flush=false' operations.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("f", "flush", "Flush target and ancestors after write (default: true)."),
	},
	Subcommands: map[string]*cmds.Command{
		"read":  FilesReadCmd,
		"write": FilesWriteCmd,
		"mv":    FilesMvCmd,
		"cp":    FilesCpCmd,
		"ls":    FilesLsCmd,
		"mkdir": FilesMkdirCmd,
		"stat":  FilesStatCmd,
		"rm":    FilesRmCmd,
		"flush": FilesFlushCmd,
	},
}

var FilesStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Display file status.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to node to stat."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(node.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		o, err := statNode(node.DAG, fsn)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(o)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*Object)
			buf := new(bytes.Buffer)
			fmt.Fprintln(buf, out.Hash)
			fmt.Fprintf(buf, "Size: %d\n", out.Size)
			fmt.Fprintf(buf, "CumulativeSize: %d\n", out.CumulativeSize)
			fmt.Fprintf(buf, "ChildBlocks: %d\n", out.Blocks)
			fmt.Fprintf(buf, "Type: %s\n", out.Type)
			return buf, nil
		},
	},
	Type: Object{},
}

func statNode(ds dag.DAGService, fsn mfs.FSNode) (*Object, error) {
	nd, err := fsn.GetNode()
	if err != nil {
		return nil, err
	}

	// add to dagserv to ensure its available
	k, err := ds.Add(nd)
	if err != nil {
		return nil, err
	}

	d, err := ft.FromBytes(nd.Data)
	if err != nil {
		return nil, err
	}

	cumulsize, err := nd.Size()
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
		Hash:           k.B58String(),
		Blocks:         len(nd.Links),
		Size:           d.GetFilesize(),
		CumulativeSize: cumulsize,
		Type:           ndtype,
	}, nil
}

var FilesCpCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Copy files into mfs.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source object to copy."),
		cmds.StringArg("dest", true, false, "Destination to copy object to."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		src, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		dst, err := checkPath(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nd, err := getNodeFromPath(req.Context(), node, src)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = mfs.PutNode(node.FilesRoot, dst, nd)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

func getNodeFromPath(ctx context.Context, node *core.IpfsNode, p string) (*dag.Node, error) {
	switch {
	case strings.HasPrefix(p, "/ipfs/"):
		np, err := path.ParsePath(p)
		if err != nil {
			return nil, err
		}

		return core.Resolve(ctx, node, np)
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

type FilesLsOutput struct {
	Entries []mfs.NodeListing
}

var FilesLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directories.",
		ShortDescription: `
List directories.

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
		cmds.StringArg("path", true, false, "Path to show listing for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("l", "Use long listing format."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(nd.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		long, _, _ := req.Option("l").Bool()

		switch fsn := fsn.(type) {
		case *mfs.Directory:
			if !long {
				var output []mfs.NodeListing
				for _, name := range fsn.ListNames() {
					output = append(output, mfs.NodeListing{
						Name: name,
					})
				}
				res.SetOutput(&FilesLsOutput{output})
			} else {
				listing, err := fsn.List()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				res.SetOutput(&FilesLsOutput{listing})
			}
			return
		case *mfs.File:
			_, name := gopath.Split(path)
			out := &FilesLsOutput{[]mfs.NodeListing{mfs.NodeListing{Name: name, Type: 1}}}
			res.SetOutput(out)
			return
		default:
			res.SetError(errors.New("unrecognized type"), cmds.ErrNormal)
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*FilesLsOutput)
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
	Type: FilesLsOutput{},
}

var FilesReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read a file in a given mfs.",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default, will
read the entire file similar to unix cat.

Examples:

    $ ipfs files read /test/hello
    hello
        `,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to file to be read."),
	},
	Options: []cmds.Option{
		cmds.IntOption("o", "offset", "Offset to read from."),
		cmds.IntOption("n", "count", "Maximum number of bytes to read."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(n.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			res.SetError(fmt.Errorf("%s was not a file", path), cmds.ErrNormal)
			return
		}

		offset, _, err := req.Option("offset").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if offset < 0 {
			res.SetError(fmt.Errorf("cannot specify negative offset"), cmds.ErrNormal)
			return
		}

		filen, err := fi.Size()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if int64(offset) > filen {
			res.SetError(fmt.Errorf("offset was past end of file (%d > %d)", offset, filen), cmds.ErrNormal)
			return
		}

		_, err = fi.Seek(int64(offset), os.SEEK_SET)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		var r io.Reader = &contextReaderWrapper{R: fi, ctx: req.Context()}
		count, found, err := req.Option("count").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if found {
			if count < 0 {
				res.SetError(fmt.Errorf("cannot specify negative 'count'"), cmds.ErrNormal)
				return
			}
			r = io.LimitReader(fi, int64(count))
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

var FilesMvCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move files.",
		ShortDescription: `
Move files around. Just like traditional unix mv.

Example:

    $ ipfs files mv /myfs/a/b/c /myfs/foo/newc

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source file to move."),
		cmds.StringArg("dest", true, false, "Target path for file to be moved to."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		src, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		dst, err := checkPath(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = mfs.Mv(n.FilesRoot, src, dst)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var FilesWriteCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write to a mutable file in a given filesystem.",
		ShortDescription: `
Write data to a file in a given filesystem. This command allows you to specify
a beginning offset to write to. The entire length of the input will be written.

If the '--create' option is specified, the file will be created if it does not
exist. Nonexistant intermediate directories will not be created.

If the '--flush' option is set to false, changes will not be propogated to the
merkledag root. This can make operations much faster when doing a large number
of writes to a deeper directory structure.

Example:

    echo "hello world" | ipfs files write --create /myfs/a/b/file
    echo "hello world" | ipfs files write --truncate /myfs/a/b/file

Warning:

    Usage of the '--flush=false' option does not guarantee data durability until
	the tree has been flushed. This can be accomplished by running 'ipfs files stat'
	on the file or any of its ancestors.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to write to."),
		cmds.FileArg("data", true, false, "Data to write.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.IntOption("o", "offset", "Offset to write to."),
		cmds.BoolOption("e", "create", "Create the file if it does not exist."),
		cmds.BoolOption("t", "truncate", "Truncate the file before writing."),
		cmds.IntOption("n", "count", "Maximum number of bytes to read."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		create, _, _ := req.Option("create").Bool()
		trunc, _, _ := req.Option("truncate").Bool()
		flush, set, _ := req.Option("flush").Bool()
		if !set {
			flush = true
		}

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		offset, _, err := req.Option("offset").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if offset < 0 {
			res.SetError(fmt.Errorf("cannot have negative write offset"), cmds.ErrNormal)
			return
		}

		fi, err := getFileHandle(nd.FilesRoot, path, create)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if flush {
			defer func() {
				fi.Close()
				err := mfs.FlushPath(nd.FilesRoot, path)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
			}()
		} else {
			defer fi.Sync()
		}

		if trunc {
			if err := fi.Truncate(0); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		count, countfound, err := req.Option("count").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if countfound && count < 0 {
			res.SetError(fmt.Errorf("cannot have negative byte count"), cmds.ErrNormal)
			return
		}

		_, err = fi.Seek(int64(offset), os.SEEK_SET)
		if err != nil {
			log.Error("seekfail: ", err)
			res.SetError(err, cmds.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var r io.Reader = input
		if countfound {
			r = io.LimitReader(r, int64(count))
		}

		n, err := io.Copy(fi, input)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		log.Debugf("wrote %d bytes to %s", n, path)
	},
}

var FilesMkdirCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Make directories.",
		ShortDescription: `
Create the directory if it does not already exist.

Note: all paths must be absolute.

Examples:

    $ ipfs mfs mkdir /test/newdir
    $ ipfs mfs mkdir -p /test/does/not/exist/yet
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to dir to make."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("p", "parents", "No error if existing, make parent directories as needed."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dashp, _, _ := req.Option("parents").Bool()
		dirtomake, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		flush, found, _ := req.Option("flush").Bool()
		if !found {
			flush = true
		}

		err = mfs.Mkdir(n.FilesRoot, dirtomake, dashp, flush)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

	},
}

var FilesFlushCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "flush a given path's data to disk",
		ShortDescription: `
flush a given path to disk. This is only useful when other commands
are run with the '--flush=false'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "path to flush (default '/')"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// take the lock and defer the unlock
		defer nd.Blockstore.PinLock()()

		path := "/"
		if len(req.Arguments()) > 0 {
			path = req.Arguments()[0]
		}

		err = mfs.FlushPath(nd.FilesRoot, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var FilesRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove a file.",
		ShortDescription: `
remove files or directories

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
		cmds.BoolOption("r", "recursive", "Recursively remove directories."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path, err := checkPath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if path == "/" {
			res.SetError(fmt.Errorf("cannot delete root"), cmds.ErrNormal)
			return
		}

		// 'rm a/b/c/' will fail unless we trim the slash at the end
		if path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}

		dir, name := gopath.Split(path)
		parent, err := mfs.Lookup(nd.FilesRoot, dir)
		if err != nil {
			res.SetError(fmt.Errorf("parent lookup: %s", err), cmds.ErrNormal)
			return
		}

		pdir, ok := parent.(*mfs.Directory)
		if !ok {
			res.SetError(fmt.Errorf("no such file or directory: %s", path), cmds.ErrNormal)
			return
		}

		dashr, _, _ := req.Option("r").Bool()

		var success bool
		defer func() {
			if success {
				err := pdir.Flush()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
			}
		}()

		// if '-r' specified, don't check file type (in bad scenarios, the block may not exist)
		if dashr {
			err := pdir.Unlink(name)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			success = true
			return
		}

		childi, err := pdir.Child(name)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		switch childi.(type) {
		case *mfs.Directory:
			res.SetError(fmt.Errorf("%s is a directory, use -r to remove directories", path), cmds.ErrNormal)
			return
		default:
			err := pdir.Unlink(name)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			success = true
		}
	},
}

func getFileHandle(r *mfs.Root, path string, create bool) (*mfs.File, error) {

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

		nd := &dag.Node{Data: ft.FilePBData(nil, 0)}
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
