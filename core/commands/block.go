package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/ipfs/go-ipfs/blocks"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/pin"
	ds "gx/ipfs/QmNgqJarToRiq2GBaPJhkmW4B5BxS5B74E1rkGvv2JoaTp/go-datastore"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
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
'ipfs block' is a plumbing command used to manipulate raw ipfs blocks.
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
on raw ipfs blocks. It outputs the following to stdout:

	Key  - the base58 encoded multihash
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to stat.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BlockStat{
			Key:  b.Key().B58String(),
			Size: len(b.RawData()),
		})
	},
	Type: BlockStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			bs := res.Output().(*BlockStat)
			return strings.NewReader(bs.String()), nil
		},
	},
}

var blockGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a raw IPFS block.",
		ShortDescription: `
'ipfs block get' is a plumbing command for retrieving raw ipfs blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to get.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(bytes.NewReader(b.RawData()))
	},
}

var blockPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw ipfs blocks.
It reads from stdin, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		file, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(file)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = file.Close()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		b := blocks.NewBlock(data)
		log.Debugf("BlockPut key: '%q'", b.Key())

		k, err := n.Blocks.AddObject(b)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BlockStat{
			Key:  k.String(),
			Size: len(data),
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			bs := res.Output().(*BlockStat)
			return strings.NewReader(bs.Key + "\n"), nil
		},
	},
	Type: BlockStat{},
}

func getBlockForKey(req cmds.Request, skey string) (blocks.Block, error) {
	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, err
	}

	if !u.IsValidHash(skey) {
		return nil, errors.New("Not a valid hash")
	}

	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	b, err := n.Blocks.GetBlock(req.Context(), c)
	if err != nil {
		return nil, err
	}

	log.Debugf("ipfs block: got block with key: %q", b.Key())
	return b, nil
}

var blockRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashs to remove.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("force", "f", "Ignore nonexistent blocks.").Default(false),
		cmds.BoolOption("quiet", "q", "Write minimal output.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		force, _, _ := req.Option("force").Bool()
		quiet, _, _ := req.Option("quiet").Bool()
		cids := make([]*cid.Cid, 0, len(hashes))
		for _, hash := range hashes {
			c, err := cid.Decode(hash)
			if err != nil {
				res.SetError(fmt.Errorf("invalid content id: %s (%s)", hash, err), cmds.ErrNormal)
				return
			}

			cids = append(cids, c)
		}
		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))
		go func() {
			defer close(outChan)
			pinning := n.Pinning
			err := rmBlocks(n.Blockstore, pinning, outChan, cids, rmBlocksOpts{
				quiet: quiet,
				force: force,
			})
			if err != nil {
				outChan <- &RemovedBlock{Error: err.Error()}
			}
		}()
		return
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)

		someFailed := false
		for out := range outChan {
			o := out.(*RemovedBlock)
			if o.Hash == "" && o.Error != "" {
				res.SetError(fmt.Errorf("aborted: %s", o.Error), cmds.ErrNormal)
				return
			} else if o.Error != "" {
				someFailed = true
				fmt.Fprintf(res.Stderr(), "cannot remove %s: %s\n", o.Hash, o.Error)
			} else {
				fmt.Fprintf(res.Stdout(), "removed %s\n", o.Hash)
			}
		}
		if someFailed {
			res.SetError(fmt.Errorf("some blocks not removed"), cmds.ErrNormal)
		}
	},
	Type: RemovedBlock{},
}

type RemovedBlock struct {
	Hash  string `json:",omitempty"`
	Error string `json:",omitempty"`
}

type rmBlocksOpts struct {
	quiet bool
	force bool
}

func rmBlocks(blocks bs.GCBlockstore, pins pin.Pinner, out chan<- interface{}, cids []*cid.Cid, opts rmBlocksOpts) error {
	unlocker := blocks.GCLock()
	defer unlocker.Unlock()

	stillOkay, err := checkIfPinned(pins, cids, out)
	if err != nil {
		return fmt.Errorf("pin check failed: %s", err)
	}

	for _, c := range stillOkay {
		err := blocks.DeleteBlock(key.Key(c.Hash()))
		if err != nil && opts.force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
			// ignore non-existent blocks
		} else if err != nil {
			out <- &RemovedBlock{Hash: c.String(), Error: err.Error()}
		} else if !opts.quiet {
			out <- &RemovedBlock{Hash: c.String()}
		}
	}
	return nil
}

func checkIfPinned(pins pin.Pinner, cids []*cid.Cid, out chan<- interface{}) ([]*cid.Cid, error) {
	stillOkay := make([]*cid.Cid, 0, len(cids))
	res, err := pins.CheckIfPinned(cids...)
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		switch r.Mode {
		case pin.NotPinned:
			stillOkay = append(stillOkay, r.Key)
		case pin.Indirect:
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: fmt.Sprintf("pinned via %s", r.Via)}
		default:
			modeStr, _ := pin.PinModeToString(r.Mode)
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: fmt.Sprintf("pinned: %s", modeStr)}

		}
	}
	return stillOkay, nil
}
