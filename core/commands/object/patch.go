package objectcmd

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/cmdutils"

	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

var ObjectPatchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to create a new merkledag object based on an existing one. Use MFS with 'files cp|rm' instead.",
		ShortDescription: `
'ipfs object patch <root> <cmd> <args>' is a plumbing command used to
build custom dag-pb objects. It mutates objects, creating new objects as a
result. This is the Merkle-DAG version of modifying an object.

DEPRECATED and provided for legacy reasons.
For modern use cases, use MFS with 'files' commands: 'ipfs files --help'.

  $ ipfs files cp /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn /some-dir
  $ ipfs files cp /ipfs/Qmayz4F4UzqcAMitTzU4zCSckDofvxstDuj3y7ajsLLEVs /some-dir/added-file.jpg
  $ ipfs files stat --hash /some-dir

  The above will add 'added-file.jpg' to the directory placed under /some-dir
  and the CID of updated directory is returned by 'files stat'

  'files cp' does not download the data, only the root block, which makes it
  possible to build arbitrary directory trees without fetching them in full to
  the local node.
`,
	},
	Arguments: []cmds.Argument{},
	Subcommands: map[string]*cmds.Command{
		"append-data": patchAppendDataCmd,
		"add-link":    patchAddLinkCmd,
		"rm-link":     patchRmLinkCmd,
		"set-data":    patchSetDataCmd,
	},
	Options: []cmds.Option{
		cmdutils.AllowBigBlockOption,
	},
}

var patchAppendDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to append data to the data segment of a DAG node.",
		ShortDescription: `
Append data to what already exists in the data segment in the given object.

Example:

	$ echo "hello" | ipfs object patch $HASH append-data

NOTE: This does not append data to a file - it modifies the actual raw
data within a dag-pb object. Blocks have a max size of 1MB and objects larger than
the limit will not be respected by the network.

DEPRECATED and provided for legacy reasons. Use 'ipfs add' or 'ipfs files' instead.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "The hash of the node to modify."),
		cmds.FileArg("data", true, false, "Data to append.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		root := path.New(req.Arguments[0])

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}

		p, err := api.Object().AppendData(req.Context, root, file)
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.Cid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.Cid().String()})
	},
	Type: &Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, obj *Object) error {
			_, err := fmt.Fprintln(w, obj.Hash)
			return err
		}),
	},
}

var patchSetDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to set the data field of dag-pb object.",
		ShortDescription: `
Set the data of an IPFS object from stdin or with the contents of a file.

Example:

    $ echo "my data" | ipfs object patch $MYHASH set-data

DEPRECATED and provided for legacy reasons. Use 'files cp' and 'dag put' instead.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "The hash of the node to modify."),
		cmds.FileArg("data", true, false, "The data to set the object to.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		root := path.New(req.Arguments[0])

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}

		p, err := api.Object().SetData(req.Context, root, file)
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.Cid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.Cid().String()})
	},
	Type: Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			fmt.Fprintln(w, out.Hash)
			return nil
		}),
	},
}

var patchRmLinkCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to remove a link from dag-pb object.",
		ShortDescription: `
Remove a Merkle-link from the given object and return the hash of the result.

DEPRECATED and provided for legacy reasons. Use 'files rm' instead.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "The hash of the node to modify."),
		cmds.StringArg("name", true, false, "Name of the link to remove."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		root := path.New(req.Arguments[0])

		name := req.Arguments[1]
		p, err := api.Object().RmLink(req.Context, root, name)
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.Cid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.Cid().String()})
	},
	Type: Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			fmt.Fprintln(w, out.Hash)
			return nil
		}),
	},
}

const (
	createOptionName = "create"
)

var patchAddLinkCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to add a link to a given dag-pb.",
		ShortDescription: `
Add a Merkle-link to the given object and return the hash of the result.

DEPRECATED and provided for legacy reasons.

Use MFS and 'files' commands instead:

  $ ipfs files cp /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn /some-dir
  $ ipfs files cp /ipfs/Qmayz4F4UzqcAMitTzU4zCSckDofvxstDuj3y7ajsLLEVs /some-dir/added-file.jpg
  $ ipfs files stat --hash /some-dir

  The above will add 'added-file.jpg' to the directory placed under /some-dir
  and the CID of updated directory is returned by 'files stat'

  'files cp' does not download the data, only the root block, which makes it
  possible to build arbitrary directory trees without fetching them in full to
  the local node.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "The hash of the node to modify."),
		cmds.StringArg("name", true, false, "Name of link to create."),
		cmds.StringArg("ref", true, false, "IPFS object to add link to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(createOptionName, "p", "Create intermediary nodes."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		root := path.New(req.Arguments[0])
		name := req.Arguments[1]
		child := path.New(req.Arguments[2])

		create, _ := req.Options[createOptionName].(bool)
		if err != nil {
			return err
		}

		p, err := api.Object().AddLink(req.Context, root, name, child,
			options.Object.Create(create))
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.Cid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.Cid().String()})
	},
	Type: Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			fmt.Fprintln(w, out.Hash)
			return nil
		}),
	},
}
