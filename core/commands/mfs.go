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

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type mfsMountListing struct {
	Mounts []mfs.RootListing
}

var MfsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate mutable filesystems",
		ShortDescription: `
Mfs is an API for manipulating ipfs objects as if they were a unix filesystem.
They can be seeded with an initial root hash, or by default are an empty directory.

NOTICE:
This API is currently experimental and likely to change and may potentially be
unstable. This notice will be removed when that is no longer the case. Feedback
on how this API could be better is very welcome.
		`,
	},
	Options: []cmds.Option{
		cmds.StringOption("s", "session", "the name of the filesystem to operate on (default=local)"),
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
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		session, found, err := req.Option("session").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if found {
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
all changes are merely propogated to the root, and are reflected in the hash
displayed by 'ipfs mfs'. In the future, this command will allow you to specify
publish actions and close actions for a given filesystem
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
			res.SetError(errNotOnline, cmds.ErrNormal)
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
			res.SetError(errNotOnline, cmds.ErrNormal)
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

func getSession(o *cmds.OptionValue) (string, error) {
	s, found, err := o.String()
	if err != nil {
		return "", err
	}
	if !found {
		s = "local"
	}

	return s, nil
}

type MfsLsOutput struct {
	Entries []mfs.NodeListing
}

var MfsLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List directories inside a filesystem",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to show listing for"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		s, found, err := req.Option("session").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			s = "local"
		}

		root, err := node.Mfs.GetRoot(s)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var dir *mfs.Directory
		switch root := root.GetValue().(type) {
		case *mfs.Directory:
			dir = root
		default:
			res.SetError(errors.New("unrecognized node type"), cmds.ErrNormal)
			return
		}

		path := req.Arguments()[0]
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		var nd mfs.FSNode
		if len(path) > 0 {
			foundnd, err := mfs.DirLookup(dir, path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			nd = foundnd
		} else {
			nd = dir
		}

		switch nd := nd.(type) {
		case *mfs.Directory:
			listing, err := nd.List()
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
			for _, o := range out.Entries {
				fmt.Fprintf(buf, "%s\n", o.Name)
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

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
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

		sess, err := getSession(req.Option("session"))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := node.Mfs.GetRoot(sess)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[1]
		err = PutNodeUnderRoot(root, path, nd)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

func PutNodeUnderRoot(root *mfs.Root, ipath string, nd *dag.Node) error {
	dir, ok := root.GetValue().(*mfs.Directory)
	if !ok {
		return errors.New("root did not point to directory")
	}
	dirp, filename := gopath.Split(ipath)

	parent, err := mfs.DirLookup(dir, dirp)
	if err != nil {
		return fmt.Errorf("lookup '%s' failed: %s", dirp, err)
	}

	pdir, ok := parent.(*mfs.Directory)
	if !ok {
		return fmt.Errorf("%s did not point to directory", dirp)
	}

	return pdir.AddChild(filename, nd)
}

var MfsReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read a file in a given mfs",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default, will
read the entire file similar to unix cat.
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
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		sess, err := getSession(req.Option("session"))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := node.Mfs.GetRoot(sess)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rdir := root.GetValue().(*mfs.Directory)

		path := req.Arguments()[0]
		fsn, err := mfs.DirLookup(rdir, path)
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

    ipfs mfs -s myfs mv /a/b/c /foo/newc

		`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "source file to move"),
		cmds.StringArg("dest", true, false, "target path for file to be moved to"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		sess, err := getSession(req.Option("session"))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := node.Mfs.GetRoot(sess)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rdir := root.GetValue().(*mfs.Directory)

		src := req.Arguments()[0]
		dst := req.Arguments()[1]
		srcDir, srcFname := gopath.Split(src)

		srcObj, err := mfs.DirLookup(rdir, src)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var dstDirStr string
		var filename string
		if dst[len(dst)-1] == '/' {
			dstDirStr = dst
			filename = srcFname
		} else {
			dstDirStr, filename = gopath.Split(dst)
		}

		dstDiri, err := mfs.DirLookup(rdir, dstDirStr)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dstDir := dstDiri.(*mfs.Directory)
		nd, err := srcObj.GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = dstDir.AddChild(filename, nd)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		srcDirObji, err := mfs.DirLookup(rdir, srcDir)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		srcDirObj := srcDirObji.(*mfs.Directory)
		err = srcDirObj.Unlink(srcFname)
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

	echo "hello world" | ipfs mfs -s myfs --create /a/b/file
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to write to"),
		cmds.FileArg("data", true, false, "data to write").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.IntOption("o", "offset", "offset to write to"),
		cmds.BoolOption("c", "create", "create the file if it does not exist"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		sess, err := getSession(req.Option("session"))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := node.Mfs.GetRoot(sess)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[0]
		create, _, _ := req.Option("create").Bool()

		fi, err := getFileHandle(root, path, create)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
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

func getFileHandle(root *mfs.Root, path string, create bool) (*mfs.File, error) {
	dir := root.GetValue().(*mfs.Directory)

	target, err := mfs.DirLookup(dir, path)
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
		pdiri, err := mfs.DirLookup(dir, dirname)
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

var MfsMkdirCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "create a new directory",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "path to dir to make"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("p", "parents", "no error if existing, make parent directories as needed"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		sess, err := getSession(req.Option("session"))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := node.Mfs.GetRoot(sess)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dashp, _, _ := req.Option("parents").Bool()

		rootdir := root.GetValue().(*mfs.Directory)

		dirtomake := req.Arguments()[0]

		parts := strings.Split(dirtomake, "/")
		if parts[0] == "" {
			parts = parts[1:]
		}

		cur := rootdir
		for i, d := range parts[:len(parts)-1] {
			fsn, err := cur.Child(d)
			if err != nil {
				if err == os.ErrNotExist && dashp {
					mkd, err := cur.Mkdir(d)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}
					fsn = mkd
				}
			}

			next, ok := fsn.(*mfs.Directory)
			if !ok {
				res.SetError(fmt.Errorf("%s was not a directory", strings.Join(parts[:i], "/")), cmds.ErrNormal)
				return
			}
			cur = next
		}

		_, err = cur.Mkdir(parts[len(parts)-1])
		if err != nil {
			if !dashp || err != os.ErrExist {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
	},
}

var MfsCmdTemplate = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Manipulate mutable filesystems",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if node.Mfs == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out := res.Output().(*mfsMountListing)
			buf := new(bytes.Buffer)
			for _, o := range out.Mounts {
				fmt.Fprintf(buf, "%s - %s\n", o.Name, o.Hash)
			}
			return buf, nil
		},
	},
	Type: mfsMountListing{},
}
