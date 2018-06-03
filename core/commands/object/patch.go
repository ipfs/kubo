package objectcmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"

	cmds "gx/ipfs/QmSKYWC84fqkKB54Te5JMcov2MBVzucXaRGxFqByzzCbHe/go-ipfs-cmds"
	logging "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var log = logging.Logger("core/commands/object")

var ObjectPatchCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new merkledag object based on an existing one.",
		ShortDescription: `
'ipfs object patch <root> <cmd> <args>' is a plumbing command used to
build custom DAG objects. It mutates objects, creating new objects as a
result. This is the Merkle-DAG version of modifying an object.
`,
	},
	Arguments: []cmdkit.Argument{},
	Subcommands: map[string]*cmds.Command{
		"append-data": patchAppendDataCmd,
		"add-link":    lgc.NewCommand(patchAddLinkCmd),
		"rm-link":     lgc.NewCommand(patchRmLinkCmd),
		"set-data":    lgc.NewCommand(patchSetDataCmd),
	},
}

func objectMarshaler(res oldcmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	o, ok := v.(*Object)
	if !ok {
		return nil, e.TypeErr(o, v)
	}

	return strings.NewReader(o.Hash + "\n"), nil
}

var patchAppendDataCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Append data to the data segment of a dag node.",
		ShortDescription: `
Append data to what already exists in the data segment in the given object.

Example:

	$ echo "hello" | ipfs object patch $HASH append-data

NOTE: This does not append data to a file - it modifies the actual raw
data within an object. Objects have a max size of 1MB and objects larger than
the limit will not be respected by the network.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("root", true, false, "The hash of the node to modify."),
		cmdkit.FileArg("data", true, false, "Data to append.").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		nd, err := GetNode(env)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := path.ParsePath(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		rootnd, err := core.Resolve(req.Context, nd.Namesys, nd.Resolver, root)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb, ok := rootnd.(*dag.ProtoNode)
		if !ok {
			re.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		fi, err := req.Files.NextFile()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(fi)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb.SetData(append(rtpb.Data(), data...))

		err = nd.DAG.Add(req.Context, rtpb)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		cmds.EmitOnce(re, &Object{Hash: rtpb.Cid().String()})
	},
	Type: Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, obj *Object) error {
			_, err := fmt.Fprintln(w, obj.Hash)
			return err
		}),
	},
}

var patchSetDataCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Set the data field of an IPFS object.",
		ShortDescription: `
Set the data of an IPFS object from stdin or with the contents of a file.

Example:

    $ echo "my data" | ipfs object patch $MYHASH set-data
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("root", true, false, "The hash of the node to modify."),
		cmdkit.FileArg("data", true, false, "The data to set the object to.").EnableStdin(),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rp, err := path.ParsePath(req.StringArguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd.Namesys, nd.Resolver, rp)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb, ok := root.(*dag.ProtoNode)
		if !ok {
			res.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		fi, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(fi)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb.SetData(data)

		err = nd.DAG.Add(req.Context(), rtpb)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: rtpb.Cid().String()})
	},
	Type: Object{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: objectMarshaler,
	},
}

var patchRmLinkCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove a link from an object.",
		ShortDescription: `
Removes a link by the given name from root.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("root", true, false, "The hash of the node to modify."),
		cmdkit.StringArg("link", true, false, "Name of the link to remove."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rootp, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd.Namesys, nd.Resolver, rootp)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb, ok := root.(*dag.ProtoNode)
		if !ok {
			res.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		path := req.Arguments()[1]

		e := dagutils.NewDagEditor(rtpb, nd.DAG)

		err = e.RmLink(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nnode, err := e.Finalize(req.Context(), nd.DAG)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nc := nnode.Cid()

		res.SetOutput(&Object{Hash: nc.String()})
	},
	Type: Object{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: objectMarshaler,
	},
}

var patchAddLinkCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add a link to a given object.",
		ShortDescription: `
Add a Merkle-link to the given object and return the hash of the result.

Example:

    $ EMPTY_DIR=$(ipfs object new unixfs-dir)
    $ BAR=$(echo "bar" | ipfs add -q)
    $ ipfs object patch $EMPTY_DIR add-link foo $BAR

This takes an empty directory, and adds a link named 'foo' under it, pointing
to a file containing 'bar', and returns the hash of the new object.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("root", true, false, "The hash of the node to modify."),
		cmdkit.StringArg("name", true, false, "Name of link to create."),
		cmdkit.StringArg("ref", true, false, "IPFS object to add link to."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("create", "p", "Create intermediary nodes."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rootp, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := core.Resolve(req.Context(), nd.Namesys, nd.Resolver, rootp)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rtpb, ok := root.(*dag.ProtoNode)
		if !ok {
			res.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		npath := req.Arguments()[1]
		childp, err := path.ParsePath(req.Arguments()[2])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		create, _, err := req.Option("create").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var createfunc func() *dag.ProtoNode
		if create {
			createfunc = ft.EmptyDirNode
		}

		e := dagutils.NewDagEditor(rtpb, nd.DAG)

		childnd, err := core.Resolve(req.Context(), nd.Namesys, nd.Resolver, childp)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = e.InsertNodeAtPath(req.Context(), npath, childnd, createfunc)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nnode, err := e.Finalize(req.Context(), nd.DAG)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nc := nnode.Cid()

		res.SetOutput(&Object{Hash: nc.String()})
	},
	Type: Object{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: objectMarshaler,
	},
}

// COPIED FROM ONE LEVEL UP
// GetNode extracts the node from the environment.
func GetNode(env interface{}) (*core.IpfsNode, error) {
	ctx, ok := env.(*oldcmds.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetNode()
}
