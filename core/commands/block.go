package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	util "github.com/ipfs/go-ipfs/blocks/blockstoreutil"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	mh "github.com/multiformats/go-multihash"
)

type BlockStat struct {
	Key  string
	Size int
}

func (bs BlockStat) String() string {
	return fmt.Sprintf("Key: %s\nSize: %d\n", bs.Key, bs.Size)
}

var BlockCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with raw IPFS blocks.",
		ShortDescription: `
'ipfs block' is a plumbing command used to manipulate raw IPFS blocks.
Reads from stdin or writes to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"stat": blockStatCmd,
		"get":  blockGetCmd,
		"put":  blockPutCmd,
		"rm":   blockRmCmd,
	},
}

var blockStatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print information of a raw IPFS block.",
		ShortDescription: `
'ipfs block stat' is a plumbing command for retrieving information
on raw IPFS blocks. It outputs the following to stdout:

	Key  - the base58 encoded multihash
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The base58 multihash of an existing block to stat.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		p, err := coreiface.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		b, err := api.Block().Stat(req.Context, p)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &BlockStat{
			Key:  b.Path().Cid().String(),
			Size: b.Size(),
		})
	},
	Type: BlockStat{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, bs *BlockStat) error {
			_, err := fmt.Fprintf(w, "%s", bs)
			return err
		}),
	},
}

var blockGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a raw IPFS block.",
		ShortDescription: `
'ipfs block get' is a plumbing command for retrieving raw IPFS blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The base58 multihash of an existing block to get.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		p, err := coreiface.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		r, err := api.Block().Get(req.Context, p)
		if err != nil {
			return err
		}

		return res.Emit(r)
	},
}

const (
	blockFormatOptionName = "format"
	mhtypeOptionName      = "mhtype"
	mhlenOptionName       = "mhlen"
)

var blockPutCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads from stdin, and <key> is a base58 encoded multihash.

By default CIDv0 is going to be generated. Setting 'mhtype' to anything other
than 'sha2-256' or format to anything other than 'v0' will result in CIDv1.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("data", true, false, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(blockFormatOptionName, "f", "cid format for blocks to be created with."),
		cmdkit.StringOption(mhtypeOptionName, "multihash hash function").WithDefault("sha2-256"),
		cmdkit.IntOption(mhlenOptionName, "multihash hash length").WithDefault(-1),
		cmdkit.BoolOption(pinOptionName, "pin added blocks recursively").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}

		mhtype, _ := req.Options[mhtypeOptionName].(string)
		mhtval, ok := mh.Names[mhtype]
		if !ok {
			return fmt.Errorf("unrecognized multihash function: %s", mhtype)
		}

		mhlen, ok := req.Options[mhlenOptionName].(int)
		if !ok {
			return errors.New("missing option \"mhlen\"")
		}

		format, formatSet := req.Options[blockFormatOptionName].(string)
		if !formatSet {
			if mhtval != mh.SHA2_256 || (mhlen != -1 && mhlen != 32) {
				format = "protobuf"
			} else {
				format = "v0"
			}
		}

		pin, _ := req.Options[pinOptionName].(bool)

		p, err := api.Block().Put(req.Context, file,
			options.Block.Hash(mhtval, mhlen),
			options.Block.Format(format),
			options.Block.Pin(pin))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &BlockStat{
			Key:  p.Path().Cid().String(),
			Size: p.Size(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, bs *BlockStat) error {
			_, err := fmt.Fprintf(w, "%s\n", bs.Key)
			return err
		}),
	},
	Type: BlockStat{},
}

const (
	forceOptionName      = "force"
	blockQuietOptionName = "quiet"
)

var blockRmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashes to remove.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(forceOptionName, "f", "Ignore nonexistent blocks."),
		cmdkit.BoolOption(blockQuietOptionName, "q", "Write minimal output."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		force, _ := req.Options[forceOptionName].(bool)
		quiet, _ := req.Options[blockQuietOptionName].(bool)

		// TODO: use batching coreapi when done
		for _, b := range req.Arguments {
			p, err := coreiface.ParsePath(b)
			if err != nil {
				return err
			}

			rp, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return err
			}

			err = api.Block().Rm(req.Context, rp, options.Block.Force(force))
			if err != nil {
				if err := res.Emit(&util.RemovedBlock{
					Hash:  rp.Cid().String(),
					Error: err.Error(),
				}); err != nil {
					return err
				}
				continue
			}

			if !quiet {
				err := res.Emit(&util.RemovedBlock{
					Hash: rp.Cid().String(),
				})
				if err != nil {
					return err
				}
			}
		}

		return nil
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			return util.ProcRmOutput(res.Next, os.Stdout, os.Stderr)
		},
	},
	Type: util.RemovedBlock{},
}
