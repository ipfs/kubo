package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strings"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mfs "github.com/ipfs/go-ipfs/mfs"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var log = u.Logger("cmds/mfs")

type mfsMountListing struct {
	Mounts []mfs.RootListing
}

var MfsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate mutable filesystems",
		ShortDescription: `
Mfs is an API for manipulating ipfs objects as if they were a unix filesystem.
They can be seeded with an initial root hash, or by default are an empty directory.

Top level mounts may be created with the create command. Once created, they show up in
the output of the base mfs command.

    $ ipfs mfs 
    demo - QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn
	files - QmdRoEYhftbSfJuYNgoy1rj9C8fF7cGLxpUkQTDCMApF5B

NOTICE:
This API is currently experimental, likely to change, and may potentially be
unstable. This notice will be removed when that is no longer the case. Feedback
on how this API could be improved is very welcome on the following issue:
https://github.com/ipfs/go-ipfs/issues/1607
`,
	},
	Subcommands: map[string]*cmds.Command{
		"create": MfsCreateCmd,
		"close":  MfsCloseCmd,
		"put":    MfsPutCmd,
		"read":   MfsReadCmd,
		"write":  MfsWriteCmd,
		"mv":     MfsMvCmd,
		"ls":     MfsLsCmd,
		"mkdir":  MfsMkdirCmd,
		"rm":     MfsRmCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", false, false, "name of filesystem to show info for"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(cmds.ErrNotOnline, cmds.ErrNormal)
			return
		}

		if len(req.Arguments()) > 0 {
			session := req.Arguments()[0]
			// if a session is specified, print out just the root hash for that fs
			root, err := node.Mfs.GetRoot(session)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			nd, err := root.GetValue().GetNode()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			k, err := nd.Key()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			res.SetOutput(&mfsMountListing{[]mfs.RootListing{mfs.RootListing{Hash: k}}})
			return
		}

		// Default: list all roots
		roots := node.Mfs.ListRoots()

		res.SetOutput(&mfsMountListing{roots})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*mfsMountListing)
			if len(out.Mounts) == 1 && out.Mounts[0].Name == "" {
				// a session was specified, print out just the hash
				return strings.NewReader(out.Mounts[0].Hash.B58String()), nil
			}

			buf := new(bytes.Buffer)
			for _, o := range out.Mounts {
				fmt.Fprintf(buf, "%s - %s\n", o.Name, o.Hash)
			}
			return buf, nil
		},
	},
	Type: mfsMountListing{},
}

var MfsCreateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new mutable filesystem",
		ShortDescription: `
Creates a new mutable filesystem based on an optional root hash.

Currently, it creates a filesystem with no special publish actions,
all changes are merely propagated to the root, and are reflected in the hash
displayed by 'ipfs mfs'.

Examples:

    $ ipfs mfs create files
	$ ipfs mfs create welcome -r QmVtU7ths96fMgZ8YSZAbKghyieq7AjxNdcqyVzxTt3qVe
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of filesystem to create"),
	},
	Options: []cmds.Option{
		cmds.StringOption("r", "root", "root object to base new filesystem on"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(cmds.ErrNotOnline, cmds.ErrNormal)
			return
		}

		name := req.Arguments()[0]
		if name == "" {
			res.SetError(errors.New("cannot have unnamed filesystem"), cmds.ErrNormal)
			return
		}

		rtkey, found, err := req.Option("root").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}

		var rootnode *dag.Node
		if found {
			k := key.B58KeyDecode(rtkey)
			if k == "" {
				res.SetError(fmt.Errorf("incorrectly formatted key: %s", rtkey), cmds.ErrNormal)
				return
			}

			nd, err := node.DAG.Get(req.Context(), k)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			rootnode = nd
		} else {
			rootnode = &dag.Node{Data: ft.FolderPBData()}
		}

		_, err = node.Mfs.NewRoot(name, rootnode, func(_ context.Context, _ key.Key) error { return nil })
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

type Object struct {
	Hash string
}

var MfsCloseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Close an open filesystem",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of filesystem to close"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(cmds.ErrNotOnline, cmds.ErrNormal)
			return
		}

		name := req.Arguments()[0]

		final, err := node.Mfs.CloseRoot(name)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: final.B58String()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*Object)
			return strings.NewReader(out.Hash), nil
		},
	},
	Type: Object{},
}

type MfsLsOutput struct {
	Entries []mfs.NodeListing
}

var MfsLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directories inside a filesystem",
		ShortDescription: `
List directories inside mfs.

Examples:

    $ mfs ls /welcome/docs/
	about
    contact
    help
    quick-start
    readme
    security-notes

	$ mfs ls /myfiles/a/b/c/d
	foo
	bar
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to show listing for"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("l", "use long listing format"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		path := req.Arguments()[0]
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fsn, err := mfs.Lookup(nd.Mfs, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		switch fsn := fsn.(type) {
		case *mfs.Directory:
			listing, err := fsn.List()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&MfsLsOutput{listing})
			return
		case *mfs.File:
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]
			out := &MfsLsOutput{[]mfs.NodeListing{mfs.NodeListing{Name: name, Type: 1}}}
			res.SetOutput(out)
			return
		default:
			res.SetError(errors.New("unrecognized type"), cmds.ErrNormal)
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*MfsLsOutput)
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
	Type: MfsLsOutput{},
}

var MfsPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "import a given file into a filesystem",
		ShortDescription: `
Mfs 'put' takes a file that already exists in ipfs and imports it into a
given mfs filesystem.

Examples:

    $ ipfs mfs create test
	$ echo "hello" | ipfs add -q
    QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
    $ ipfs mfs put QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN /test/hello
		`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("object", true, false, "ipfspath of object to import"),
		cmds.StringArg("path", true, false, "path within filesystem to import to"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		obj := req.Arguments()[0]
		objpath, err := path.ParsePath(obj)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nd, err := node.Resolver.ResolvePath(req.Context(), objpath)
		if err != nil {
			res.SetError(fmt.Errorf("resolve path failed: %s", err), cmds.ErrNormal)
			return
		}

		path := req.Arguments()[1]
		err = mfs.PutNode(node.Mfs, path, nd)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var MfsReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read a file in a given mfs",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default, will
read the entire file similar to unix cat.

Examples:

    $ ipfs mfs read /test/hello
	hello
		`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to file to be read"),
	},
	Options: []cmds.Option{
		cmds.IntOption("o", "offset", "offset to read from"),
		cmds.IntOption("n", "count", "maximum number of bytes to read"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[0]
		fsn, err := mfs.Lookup(n.Mfs, path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			res.SetError(fmt.Errorf("%s was not a file", path), cmds.ErrNormal)
			return
		}

		offset, _, _ := req.Option("offset").Int()

		_, err = fi.Seek(int64(offset), os.SEEK_SET)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		var r io.Reader = fi
		count, found, err := req.Option("count").Int()
		if err == nil && found {
			r = io.LimitReader(fi, int64(count))
		}

		res.SetOutput(r)
	},
}

var MfsMvCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move files",
		ShortDescription: `
Move files around. Just like traditional unix mv.

Example:

    $ ipfs mfs mv /myfs/a/b/c /myfs/foo/newc

		`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "source file to move"),
		cmds.StringArg("dest", true, false, "target path for file to be moved to"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		src := req.Arguments()[0]
		dst := req.Arguments()[1]

		err = mfs.Mv(n.Mfs, src, dst)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var MfsWriteCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write to a mutable file in a given filesystem",
		ShortDescription: `
Write data to a file in a given filesystem. This command allows you to specify
a beginning offset to write to. The entire length of the input will be written.

If the '--create' option is specified, the file will be create if it does not
exist. Nonexistant intermediate directories will not be created.

Example:

	echo "hello world" | ipfs mfs --create /myfs/a/b/file
	echo "hello world" | ipfs mfs --truncate /myfs/a/b/file
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to write to"),
		cmds.FileArg("data", true, false, "data to write").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.IntOption("o", "offset", "offset to write to"),
		cmds.BoolOption("c", "create", "create the file if it does not exist"),
		cmds.BoolOption("t", "truncate", "truncate the file before writing"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		path := req.Arguments()[0]
		create, _, _ := req.Option("create").Bool()
		trunc, _, _ := req.Option("truncate").Bool()

		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, err := getFileHandle(nd.Mfs, path, create)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if trunc {
			if err := fi.Truncate(0); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		offset, _, _ := req.Option("offset").Int()

		_, err = fi.Seek(int64(offset), os.SEEK_SET)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		n, err := io.Copy(fi, input)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		log.Debugf("wrote %d bytes to %s", n, path)
	},
}

var MfsMkdirCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "make directories",
		ShortDescription: `
Create the directory if it does not already exist

Examples:

    $ ipfs mfs mkdir /test/newdir
	$ ipfs mfs mkdir -p /test/does/not/exist/yet
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to dir to make"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("p", "parents", "no error if existing, make parent directories as needed"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dashp, _, _ := req.Option("parents").Bool()
		dirtomake := req.Arguments()[0]

		err = mfs.Mkdir(n.Mfs, dirtomake, dashp)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var MfsRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "remove a file",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, true, "file to remove"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("r", "recursive", "recursively remove directories"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[0]
		dir, name := gopath.Split(path)
		parent, err := mfs.Lookup(nd.Mfs, dir)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pdir, ok := parent.(*mfs.Directory)
		if !ok {
			res.SetError(fmt.Errorf("no such file or directory: %s", path), cmds.ErrNormal)
			return
		}

		childi, err := pdir.Child(name)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dashr, _, _ := req.Option("r").Bool()

		switch childi.(type) {
		case *mfs.Directory:
			if dashr {
				err := pdir.Unlink(name)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
			} else {
				res.SetError(fmt.Errorf("%s is a directory, use -r to remove directories", path), cmds.ErrNormal)
				return
			}
		default:
			err := pdir.Unlink(name)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
	},
}

func getFileHandle(fs *mfs.Filesystem, path string, create bool) (*mfs.File, error) {

	target, err := mfs.Lookup(fs, path)
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
		pdiri, err := mfs.Lookup(fs, dirname)
		if err != nil {
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

		// can unsafely cast, if it fails, that means programmer error
		return fsn.(*mfs.File), nil

	default:
		return nil, err
	}
}
