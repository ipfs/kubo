package objectcmd

import (
	"fmt"
	"io"
	"strings"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"
)

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
		api, err := GetApi(env)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := coreiface.ParsePath(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := req.Files.NextFile()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := api.Object().AppendData(req.Context, root, data)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		cmds.EmitOnce(re, &Object{Hash: p.Cid().String()})
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
		api, err := req.InvocContext().GetApi()

		root, err := coreiface.ParsePath(req.StringArguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := api.Object().SetData(req.Context(), root, data)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: p.Cid().String()})
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
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		link := req.Arguments()[1]
		p, err := api.Object().RmLink(req.Context(), root, link)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: p.Cid().String()})
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
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		root, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		name := req.Arguments()[1]

		child, err := coreiface.ParsePath(req.Arguments()[2])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		create, _, err := req.Option("create").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := api.Object().AddLink(req.Context(), root, name, child,
			options.Object.Create(create))
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: p.Cid().String()})
	},
	Type: Object{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: objectMarshaler,
	},
}

// TODO: fix import loop with core/commands so we don't need that
// COPIED FROM ONE LEVEL UP
// GetApi extracts CoreAPI instance from the environment.
func GetApi(env cmds.Environment) (coreiface.CoreAPI, error) {
	ctx, ok := env.(*oldcmds.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetApi()
}
