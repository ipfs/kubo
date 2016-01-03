package objectcmd

import (
	"io"
	"io/ioutil"
	"strings"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

var ObjectPatchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new merkledag object based on an existing one",
		ShortDescription: `
'ipfs object patch <root> <cmd> <args>' is a plumbing command used to
build custom DAG objects. It adds and removes links from objects, creating a new
object as a result. This is the merkle-dag version of modifying an object. It
can also set the data inside a node with 'set-data' and append to that data as
well with 'append-data'.

Patch commands:
    add-link <name> <ref>     - adds a link to a node
    rm-link <name>            - removes a link from a node
    set-data                  - sets a nodes data from stdin
    append-data               - appends to a nodes data from stdin



    ipfs object patch $FOO_BAR rm-link foo

This removes the link named foo from the hash in $FOO_BAR and returns the
resulting object hash.

The data inside the node can be modified as well:

    ipfs object patch $FOO_BAR set-data < file.dat
    ipfs object patch $FOO_BAR append-data < file.dat

`,
	},
	Arguments: []cmds.Argument{},
	Subcommands: map[string]*cmds.Command{
		"append-data": patchAppendDataCmd,
		"add-link":    patchAddLinkCmd,
		"rm-link":     patchRmLinkCmd,
		"set-data":    patchSetDataCmd,
	},
}

func objectMarshaler(res cmds.Response) (io.Reader, error) {
	o, ok := res.Output().(*Object)
	if !ok {
		return nil, u.ErrCast()
	}

	return strings.NewReader(o.Hash + "\n"), nil
}

var patchAppendDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Append data to the data segment of a dag node",
		ShortDescription: `
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "the hash of the node to modify"),
		cmds.FileArg("data", true, false, "data to append").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rootnd, err := core.Resolve(req.Context(), nd, root)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(req.Files())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rootnd.Data = append(rootnd.Data, data...)

		newkey, err := nd.DAG.Add(rootnd)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: newkey.B58String()})
	},
	Type: Object{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: objectMarshaler,
	},
}

var patchSetDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "the hash of the node to modify"),
		cmds.FileArg("data", true, false, "data fill with").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rp, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd, rp)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(req.Files())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root.Data = data

		newkey, err := nd.DAG.Add(root)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: newkey.B58String()})
	},
	Type: Object{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: objectMarshaler,
	},
}

var patchRmLinkCmd = &cmds.Command{
	Helptext: cmds.HelpText{},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "the hash of the node to modify"),
		cmds.StringArg("link", true, false, "name of the link to remove"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rootp, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd, rootp)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[1]

		e := dagutils.NewDagEditor(root, nd.DAG)

		err = e.RmLink(req.Context(), path)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nnode, err := e.Finalize(nd.DAG)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nk, err := nnode.Key()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: nk.B58String()})
	},
	Type: Object{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: objectMarshaler,
	},
}

var patchAddLinkCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "add a link to a given object",
		ShortDescription: `
Examples:

    EMPTY_DIR=$(ipfs object new unixfs-dir)
    BAR=$(echo "bar" | ipfs add -q)
    ipfs object patch $EMPTY_DIR add-link foo $BAR

This takes an empty directory, and adds a link named foo under it, pointing to
a file containing 'bar', and returns the hash of the new object.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("p", "create", "create intermediary nodes"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "the hash of the node to modify"),
		cmds.StringArg("name", true, false, "name of link to create"),
		cmds.StringArg("ref", true, false, "ipfs object to add link to"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rootp, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd, rootp)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		path := req.Arguments()[1]
		childk := key.B58KeyDecode(req.Arguments()[2])

		create, _, err := req.Option("create").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var createfunc func() *dag.Node
		if create {
			createfunc = func() *dag.Node {
				return &dag.Node{Data: ft.FolderPBData()}
			}
		}

		e := dagutils.NewDagEditor(root, nd.DAG)

		childnd, err := nd.DAG.Get(req.Context(), childk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = e.InsertNodeAtPath(req.Context(), path, childnd, createfunc)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nnode, err := e.Finalize(nd.DAG)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nk, err := nnode.Key()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: nk.B58String()})
	},
	Type: Object{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: objectMarshaler,
	},
}
