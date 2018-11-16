package dagcmd

import (
	"fmt"
	"io"
	"math"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coredag"
	"github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	path "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path"
	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
)

var DagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     cid.Cid
	RemPath string
}

var DagPutCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "f", "Format that the object will be added as.").WithDefault("cbor"),
		cmdkit.StringOption("input-enc", "Format that the input object will be.").WithDefault("json"),
		cmdkit.BoolOption("pin", "Pin this object when adding."),
		cmdkit.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		ienc, _ := req.Options["input-enc"].(string)
		format, _ := req.Options["format"].(string)
		hash, _ := req.Options["hash"].(string)
		dopin, _ := req.Options["pin"].(bool)

		// mhType tells inputParser which hash should be used. MaxUint64 means 'use
		// default hash' (sha256 for cbor, sha1 for git..)
		mhType := uint64(math.MaxUint64)

		if hash != "" {
			var ok bool
			mhType, ok = mh.Names[hash]
			if !ok {
				return fmt.Errorf("%s in not a valid multihash name", hash)
			}
		}

		cids := cid.NewSet()
		b := ipld.NewBatch(req.Context, nd.DAG)

		if dopin {
			defer nd.Blockstore.PinLock().Unlock()
		}

		for {
			file, err := req.Files.NextFile()
			if err == io.EOF {
				// Finished the list of files.
				break
			} else if err != nil {
				return err
			}

			nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
			if err != nil {
				return err
			}
			if len(nds) == 0 {
				return fmt.Errorf("no node returned from ParseInputs")
			}

			for _, nd := range nds {
				err := b.Add(req.Context, nd)
				if err != nil {
					return err
				}
			}

			cid := nds[0].Cid()
			cids.Add(cid)
			if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
				return err
			}
		}

		if err := b.Commit(); err != nil {
			return err
		}

		if dopin {
			cids.ForEach(func(c cid.Cid) error {
				nd.Pinning.PinWithMode(c, pin.Recursive)
				return nil
			})

			err := nd.Pinning.Flush()
			if err != nil {
				return err
			}
		}
		return nil
	},
	Type: OutputObject{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OutputObject) error {
			fmt.Fprintln(w, out.Cid.String())
			return nil
		}),
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specified
format.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		p, err := path.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		lastCid, rem, err := nd.Resolver.ResolveToLastNode(req.Context, p)
		if err != nil {
			return err
		}
		obj, err := nd.DAG.Get(req.Context, lastCid)
		if err != nil {
			return err
		}

		var out interface{} = obj
		if len(rem) > 0 {
			final, _, err := obj.Resolve(rem)
			if err != nil {
				return err
			}
			out = final
		}
		return cmds.EmitOnce(res, &out)
	},
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Resolve ipld block",
		ShortDescription: `
'ipfs dag resolve' fetches a dag node from ipfs, prints it's address and remaining path.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		p, err := path.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		lastCid, rem, err := nd.Resolver.ResolveToLastNode(req.Context, p)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ResolveOutput{
			Cid:     lastCid,
			RemPath: path.Join(rem),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ResolveOutput) error {
			p := out.Cid.String()
			if out.RemPath != "" {
				p = path.Join([]string{p, out.RemPath})
			}

			fmt.Fprint(w, p)
			return nil
		}),
	},
	Type: ResolveOutput{},
}
