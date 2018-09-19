package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	util "github.com/ipfs/go-ipfs/blocks/blockstoreutil"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cmds "gx/ipfs/QmPXR4tNdLbp8HsZiPMjpsgqphX9Vhw2J6Jh5MKH2ovW3D/go-ipfs-cmds"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
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
		api, err := cmdenv.GetApi(env)
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
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			bs, ok := v.(*BlockStat)
			if !ok {
				return e.TypeErr(bs, v)
			}
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
		api, err := cmdenv.GetApi(env)
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
		cmdkit.StringOption("format", "f", "cid format for blocks to be created with."),
		cmdkit.StringOption("mhtype", "multihash hash function").WithDefault("sha2-256"),
		cmdkit.IntOption("mhlen", "multihash hash length").WithDefault(-1),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		file, err := req.Files.NextFile()
		if err != nil {
			return err
		}

		mhtype, _ := req.Options["mhtype"].(string)
		mhtval, ok := mh.Names[mhtype]
		if !ok {
			return fmt.Errorf("unrecognized multihash function: %s", mhtype)
		}

		mhlen, ok := req.Options["mhlen"].(int)
		if !ok {
			return errors.New("missing option \"mhlen\"")
		}

		format, formatSet := req.Options["format"].(string)
		if !formatSet {
			if mhtval != mh.SHA2_256 || (mhlen != -1 && mhlen != 32) {
				format = "protobuf"
			} else {
				format = "v0"
			}
		}

		p, err := api.Block().Put(req.Context, file, options.Block.Hash(mhtval, mhlen), options.Block.Format(format))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &BlockStat{
			Key:  p.Path().Cid().String(),
			Size: p.Size(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			bs, ok := v.(*BlockStat)
			if !ok {
				return e.TypeErr(bs, v)
			}
			_, err := fmt.Fprintf(w, "%s\n", bs.Key)
			return err
		}),
	},
	Type: BlockStat{},
}

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
		cmdkit.BoolOption("force", "f", "Ignore nonexistent blocks."),
		cmdkit.BoolOption("quiet", "q", "Write minimal output."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		force, _ := req.Options["force"].(bool)
		quiet, _ := req.Options["quiet"].(bool)

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
				err := res.Emit(&util.RemovedBlock{
					Hash:  rp.Cid().String(),
					Error: err.Error(),
				})
				if err != nil {
					return err
				}
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
