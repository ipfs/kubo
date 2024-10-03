package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/boxo/files"

	"github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	options "github.com/ipfs/kubo/core/coreiface/options"

	cmds "github.com/ipfs/go-ipfs-cmds"
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
Reads from stdin or writes to stdout. A block is identified by a Multihash
passed with a valid CID.
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

	Key  - the CID of the block
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "The CID of an existing block to stat.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		p, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		b, err := api.Block().Stat(req.Context, p)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &BlockStat{
			Key:  b.Path().RootCid().String(),
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
It takes a <cid>, and outputs the block to stdout.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "The CID of an existing block to get.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		p, err := cmdutils.PathOrCidPath(req.Arguments[0])
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
	blockFormatOptionName   = "format"
	blockCidCodecOptionName = "cid-codec"
	mhtypeOptionName        = "mhtype"
	mhlenOptionName         = "mhlen"
)

var blockPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads data from stdin, and outputs the block's CID to stdout.

Unless cid-codec is specified, this command returns raw (0x55) CIDv1 CIDs.

Passing alternative --cid-codec does not modify imported data, nor run any
validation. It is provided solely for convenience for users who create blocks
in userland.

NOTE:
Do not use --format for any new code. It got superseded by --cid-codec and left
only for backward compatibility when a legacy CIDv0 is required (--format=v0).
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, true, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(blockCidCodecOptionName, "Multicodec to use in returned CID").WithDefault("raw"),
		cmds.StringOption(mhtypeOptionName, "Multihash hash function"),
		cmds.IntOption(mhlenOptionName, "Multihash hash length").WithDefault(-1),
		cmds.BoolOption(pinOptionName, "Pin added blocks recursively").WithDefault(false),
		cmdutils.AllowBigBlockOption,
		cmds.StringOption(blockFormatOptionName, "f", "Use legacy format for returned CID (DEPRECATED)"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		cfg, err := nd.Repo.Config()
		if err != nil {
			return err
		}

		mhtype, _ := req.Options[mhtypeOptionName].(string)
		if mhtype == "" {
			mhtype = cfg.Import.HashFunction.WithDefault(config.DefaultHashFunction)
		}

		mhtval, ok := mh.Names[mhtype]
		if !ok {
			return fmt.Errorf("unrecognized multihash function: %s", mhtype)
		}

		mhlen, ok := req.Options[mhlenOptionName].(int)
		if !ok {
			return errors.New("missing option \"mhlen\"")
		}

		cidCodec, _ := req.Options[blockCidCodecOptionName].(string)
		format, _ := req.Options[blockFormatOptionName].(string) // deprecated

		// use of legacy 'format' needs to suppress 'cid-codec'
		if format != "" {
			if cidCodec != "" && cidCodec != "raw" {
				return fmt.Errorf("unable to use %q (deprecated) and a custom %q at the same time", blockFormatOptionName, blockCidCodecOptionName)
			}
			cidCodec = "" // makes it no-op
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
				options.Block.CidCodec(cidCodec),
				options.Block.Format(format),
				options.Block.Pin(pin))
			if err != nil {
				return err
			}

			if err := cmdutils.CheckBlockSize(req, uint64(p.Size())); err != nil {
				return err
			}

			err = res.Emit(&BlockStat{
				Key:  p.Path().RootCid().String(),
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

type removedBlock struct {
	Hash  string `json:",omitempty"`
	Error string `json:",omitempty"`
}

var blockRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove IPFS block(s) from the local datastore.",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of CIDs to remove from the local datastore..
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CIDs of block(s) to remove."),
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
			p, err := cmdutils.PathOrCidPath(b)
			if err != nil {
				return err
			}

			rp, _, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return err
			}

			err = api.Block().Rm(req.Context, rp, options.Block.Force(force))
			if err != nil {
				if err := res.Emit(&removedBlock{
					Hash:  rp.RootCid().String(),
					Error: err.Error(),
				}); err != nil {
					return err
				}
				continue
			}

			if !quiet {
				err := res.Emit(&removedBlock{
					Hash: rp.RootCid().String(),
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
			someFailed := false
			for {
				res, err := res.Next()
				if err == io.EOF {
					break
				} else if err != nil {
					return err
				}
				r := res.(*removedBlock)
				if r.Hash == "" && r.Error != "" {
					return fmt.Errorf("aborted: %s", r.Error)
				} else if r.Error != "" {
					someFailed = true
					fmt.Fprintf(os.Stderr, "cannot remove %s: %s\n", r.Hash, r.Error)
				} else {
					fmt.Fprintf(os.Stdout, "removed %s\n", r.Hash)
				}
			}
			if someFailed {
				return fmt.Errorf("some blocks not removed")
			}
			return nil
		},
	},
	Type: removedBlock{},
}
