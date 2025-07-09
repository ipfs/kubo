package objectcmd

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	"github.com/ipfs/kubo/core/coreiface/options"
)

var ObjectPatchCmd = &cmds.Command{
	Status: cmds.Deprecated, // https://github.com/ipfs/kubo/issues/7936
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
		"append-data": RemovedObjectCmd,
		"add-link":    patchAddLinkCmd,
		"rm-link":     patchRmLinkCmd,
		"set-data":    RemovedObjectCmd,
	},
	Options: []cmds.Option{
		cmdutils.AllowBigBlockOption,
	},
}

var patchRmLinkCmd = &cmds.Command{
	Status: cmds.Deprecated, // https://github.com/ipfs/kubo/issues/7936
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

		root, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		name := req.Arguments[1]
		p, err := api.Object().RmLink(req.Context, root, name)
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.RootCid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.RootCid().String()})
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
	Status: cmds.Deprecated, // https://github.com/ipfs/kubo/issues/7936
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

		root, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		name := req.Arguments[1]

		child, err := cmdutils.PathOrCidPath(req.Arguments[2])
		if err != nil {
			return err
		}

		create, _ := req.Options[createOptionName].(bool)
		if err != nil {
			return err
		}

		p, err := api.Object().AddLink(req.Context, root, name, child,
			options.Object.Create(create))
		if err != nil {
			return err
		}

		if err := cmdutils.CheckCIDSize(req, p.RootCid(), api.Dag()); err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: p.RootCid().String()})
	},
	Type: Object{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			fmt.Fprintln(w, out.Hash)
			return nil
		}),
	},
}
