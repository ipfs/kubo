package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	util "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"gx/ipfs/QmP9vZfc5WSjfGTXmwX2EcicMFzmZ6fXn7HTdKYat6ccmH/go-ipfs-cmds"
	"gx/ipfs/QmQp2a2Hhb7F6eK2A5hN8f9aJy4mtkEikL9Zj4cgB7d1dD/go-ipfs-cmdkit"

	mh "gx/ipfs/QmYeKnKpubCMRiq3PGZcTREErthbb5Q9cXsCoSkD9bjEBd/go-multihash"
	blocks "gx/ipfs/QmYsEQydGrsxNZfAiskvQ76N2xE9hDQtSAkRSynwMiUK3c/go-block-format"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
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
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = cmds.EmitOnce(res, &BlockStat{
			Key:  b.Cid().String(),
			Size: len(b.RawData()),
		})
		if err != nil {
			log.Error(err)
		}
	},
	Type: BlockStat{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req cmds.Request, w io.Writer, v interface{}) error {
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
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = res.Emit(bytes.NewReader(b.RawData()))
		if err != nil {
			log.Error(err)
		}
	},
}

var blockPutCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads from stdin, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("data", true, false, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "f", "cid format for blocks to be created with.").WithDefault("v0"),
		cmdkit.StringOption("mhtype", "multihash hash function").WithDefault("sha2-256"),
		cmdkit.IntOption("mhlen", "multihash hash length").WithDefault(-1),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		file, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(file)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = file.Close()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var pref cid.Prefix
		pref.Version = 1

		format, _, _ := req.Option("format").String()
		formatval, ok := cid.Codecs[format]
		if !ok {
			res.SetError(fmt.Errorf("unrecognized format: %s", format), cmdkit.ErrNormal)
			return
		}
		if format == "v0" {
			pref.Version = 0
		}
		pref.Codec = formatval

		mhtype, _, _ := req.Option("mhtype").String()
		mhtval, ok := mh.Names[mhtype]
		if !ok {
			err := fmt.Errorf("unrecognized multihash function: %s", mhtype)
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		pref.MhType = mhtval

		mhlen, _, err := req.Option("mhlen").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		pref.MhLength = mhlen

		bcid, err := pref.Sum(data)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		b, err := blocks.NewBlockWithCid(data, bcid)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		k, err := n.Blocks.AddBlock(b)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = cmds.EmitOnce(res, &BlockStat{
			Key:  k.String(),
			Size: len(data),
		})
		if err != nil {
			log.Error(err)
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req cmds.Request, w io.Writer, v interface{}) error {
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

func getBlockForKey(req cmds.Request, skey string) (blocks.Block, error) {
	if len(skey) == 0 {
		return nil, fmt.Errorf("zero length cid invalid")
	}

	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, err
	}

	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	b, err := n.Blocks.GetBlock(req.Context(), c)
	if err != nil {
		return nil, err
	}

	return b, nil
}

var blockRmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashs to remove.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("force", "f", "Ignore nonexistent blocks."),
		cmdkit.BoolOption("quiet", "q", "Write minimal output."),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		hashes := req.Arguments()
		force, _, _ := req.Option("force").Bool()
		quiet, _, _ := req.Option("quiet").Bool()
		cids := make([]*cid.Cid, 0, len(hashes))
		for _, hash := range hashes {
			c, err := cid.Decode(hash)
			if err != nil {
				err = fmt.Errorf("invalid content id: %s (%s)", hash, err)
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			cids = append(cids, c)
		}
		ch, err := util.RmBlocks(n.Blockstore, n.Pinning, cids, util.RmBlocksOpts{
			Quiet: quiet,
			Force: force,
		})

		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = res.Emit(ch)
		if err != nil {
			log.Error(err)
		}
	},
	PostRun: map[cmds.EncodingType]func(cmds.Request, cmds.ResponseEmitter) cmds.ResponseEmitter{
		cmds.CLI: func(req cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
			reNext, res := cmds.NewChanResponsePair(req)

			go func() {
				defer re.Close()

				err := util.ProcRmOutput(res.Next, os.Stdout, os.Stderr)
				cmds.HandleError(err, res, re)
			}()

			return reNext
		},
	},
	Type: util.RemovedBlock{},
}
