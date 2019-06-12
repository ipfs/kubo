package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	files "github.com/ipfs/go-ipfs-files"

	util "github.com/ipfs/go-ipfs/blocks/blockstoreutil"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
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
	Helptext: cmds.HelpText{
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
	Helptext: cmds.HelpText{
		Tagline: "Print information of a raw IPFS block.",
		ShortDescription: `
'ipfs block stat' is a plumbing command for retrieving information
on raw IPFS blocks. It outputs the following to stdout:

	Key  - the base58 encoded multihash
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to stat.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		b, err := api.Block().Stat(req.Context, path.New(req.Arguments[0]))
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
	Helptext: cmds.HelpText{
		Tagline: "Get a raw IPFS block.",
		ShortDescription: `
'ipfs block get' is a plumbing command for retrieving raw IPFS blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to get.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		r, err := api.Block().Get(req.Context, path.New(req.Arguments[0]))
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
	Helptext: cmds.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads from stdin, and outputs the block's CID to stdout.

Unless specified, this command returns dag-pb CIDv0 CIDs. Setting 'mhtype' to anything
other than 'sha2-256' or format to anything other than 'v0' will result in CIDv1.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, true, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(blockFormatOptionName, "f", "cid format for blocks to be created with."),
		cmds.StringOption(mhtypeOptionName, "multihash hash function").WithDefault("sha2-256"),
		cmds.IntOption(mhlenOptionName, "multihash hash length").WithDefault(-1),
		cmds.BoolOption(pinOptionName, "pin added blocks recursively").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
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

		it := req.Files.Entries()
		for it.Next() {
			file := files.FileFromEntry(it)
			if file == nil {
				return errors.New("expected a file")
			}

			p, err := api.Block().Put(req.Context, file,
				options.Block.Hash(mhtval, mhlen),
				options.Block.Format(format),
				options.Block.Pin(pin))
			if err != nil {
				return err
			}

			err = res.Emit(&BlockStat{
				Key:  p.Path().Cid().String(),
				Size: p.Size(),
			})
			if err != nil {
				return err
			}
		}

		return it.Err()
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
	Helptext: cmds.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashes to remove.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(forceOptionName, "f", "Ignore nonexistent blocks."),
		cmds.BoolOption(blockQuietOptionName, "q", "Write minimal output."),
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
			rp, err := api.ResolvePath(req.Context, path.New(b))
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
