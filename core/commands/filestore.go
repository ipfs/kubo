package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	oldCmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/filestore"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	"gx/ipfs/QmUyfy4QSr3NXym4etEiRyxBLqqAeKHJuRdi8AACxg63fZ/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmamUWYjFeYYzFDFPTvnmGkozJigsoDWUA4zoifTRFTnwK/go-ipfs-cmds"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": lsFileStore,
		"rm": rmFileStore,
	},
	OldSubcommands: map[string]*oldCmds.Command{
		"verify": verifyFileStore,
		"dups":   dupsFileStore,
	},
}

type lsEncoder struct {
	errors bool
	w      io.Writer
}

var lsFileStore = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List objects in filestore.",
		LongDescription: `
List objects in the filestore.

If one or more <obj> is specified only list those specific objects,
otherwise list all objects.

The output is:

<hash> <size> <path> <offset>
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("obj", false, true, "Cid of objects to list."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("file-order", "sort the results based on the path of the backing file"),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		_, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		args := req.Arguments()
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context(), args, func(c *cid.Cid) *filestore.ListRes {
				return filestore.List(fs, c)
			})

			err = res.Emit(out)
			if err != nil {
				log.Error(err)
			}
		} else {
			fileOrder, _, _ := req.Option("file-order").Bool()
			next, err := filestore.ListAll(fs, fileOrder)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			out := listResToChan(req.Context(), next)
			err = res.Emit(out)
			if err != nil {
				log.Error(err)
			}
		}
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(req cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
			reNext, res := cmds.NewChanResponsePair(req)

			go func() {
				defer re.Close()

				var errors bool
				for {
					v, err := res.Next()
					if !cmds.HandleError(err, res, re) {
						break
					}

					r, ok := v.(*filestore.ListRes)
					if !ok {
						log.Error(e.New(e.TypeErr(r, v)))
						return
					}

					if r.ErrorMsg != "" {
						errors = true
						fmt.Fprintf(os.Stderr, "%s\n", r.ErrorMsg)
					} else {
						fmt.Fprintf(os.Stdout, "%s\n", r.FormatLong())
					}
				}

				if errors {
					re.SetError("errors while displaying some entries", cmdkit.ErrNormal)
				}
			}()

			return reNext
		},
	},
	Type: filestore.ListRes{},
}

var verifyFileStore = &oldCmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Verify objects in filestore.",
		LongDescription: `
Verify objects in the filestore.

If one or more <obj> is specified only verify those specific objects,
otherwise verify all objects.

The output is:

<status> <hash> <size> <path> <offset>

Where <status> is one of:
ok:       the block can be reconstructed
changed:  the contents of the backing file have changed
no-file:  the backing file could not be found
error:    there was some other problem reading the file
missing:  <obj> could not be found in the filestore
ERROR:    internal error, most likely due to a corrupt database

For ERROR entries the error will also be printed to stderr.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("obj", false, true, "Cid of objects to verify."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("file-order", "verify the objects based on the order of the backing file"),
	},
	Run: func(req oldCmds.Request, res oldCmds.Response) {
		_, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		args := req.Arguments()
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context(), args, func(c *cid.Cid) *filestore.ListRes {
				return filestore.Verify(fs, c)
			})
			res.SetOutput(out)
		} else {
			fileOrder, _, _ := req.Option("file-order").Bool()
			next, err := filestore.VerifyAll(fs, fileOrder)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			out := listResToChan(req.Context(), next)
			res.SetOutput(out)
		}
	},
	Marshalers: oldCmds.MarshalerMap{
		oldCmds.Text: func(res oldCmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			r, ok := v.(*filestore.ListRes)
			if !ok {
				return nil, e.TypeErr(r, v)
			}

			if r.Status == filestore.StatusOtherError {
				fmt.Fprintf(res.Stderr(), "%s\n", r.ErrorMsg)
			}
			fmt.Fprintf(res.Stdout(), "%s %s\n", r.Status.Format(), r.FormatLong())
			return nil, nil
		},
	},
	Type: filestore.ListRes{},
}

var dupsFileStore = &oldCmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List blocks that are both in the filestore and standard block storage.",
	},
	Run: func(req oldCmds.Request, res oldCmds.Response) {
		_, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		ch, err := fs.FileManager().AllKeysChan(req.Context())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		out := make(chan interface{}, 128)
		res.SetOutput((<-chan interface{})(out))

		go func() {
			defer close(out)
			for cid := range ch {
				have, err := fs.MainBlockstore().Has(cid)
				if err != nil {
					out <- &RefWrapper{Err: err.Error()}
					return
				}
				if have {
					out <- &RefWrapper{Ref: cid.String()}
				}
			}
		}()
	},
	Marshalers: refsMarshallerMap,
	Type:       RefWrapper{},
}

var rmFileStore = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove IPFS block(s) from just the filestore or blockstore.",
		ShortDescription: `
Remove blocks from either the filestore or the main blockstore.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("hash", true, true, "CID's of block(s) to remove."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("force", "f", "Ignore nonexistent blocks."),
		cmdkit.BoolOption("quiet", "q", "Write minimal output."),
		cmdkit.BoolOption("non-filestore", "Remove non-filestore blocks"),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		n, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		hashes := req.Arguments()
		force, _, _ := req.Option("force").Bool()
		quiet, _, _ := req.Option("quiet").Bool()
		nonFilestore, _, _ := req.Option("non-filestore").Bool()
		prefix := filestore.FilestorePrefix.String()
		if nonFilestore {
			prefix = bs.BlockPrefix.String()
		}
		cids := make([]*cid.Cid, 0, len(hashes))
		for _, hash := range hashes {
			c, err := cid.Decode(hash)
			if err != nil {
				res.SetError(fmt.Errorf("invalid content id: %s (%s)", hash, err), cmdkit.ErrNormal)
				return
			}

			cids = append(cids, c)
		}
		ch, err := filestore.RmBlocks(fs, n.Blockstore, n.Pinning, cids, butil.RmBlocksOpts{
			Prefix: prefix,
			Quiet:  quiet,
			Force:  force,
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
	PostRun: blockRmCmd.PostRun,
	Type:    butil.RemovedBlock{},
}

type getNoder interface {
	GetNode() (*core.IpfsNode, error)
}

func getFilestore(g getNoder) (*core.IpfsNode, *filestore.Filestore, error) {
	n, err := g.GetNode()
	if err != nil {
		return nil, nil, err
	}
	fs := n.Filestore
	if fs == nil {
		return n, nil, fmt.Errorf("filestore not enabled")
	}
	return n, fs, err
}

func listResToChan(ctx context.Context, next func() *filestore.ListRes) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for {
			r := next()
			if r == nil {
				return
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func perKeyActionToChan(ctx context.Context, args []string, action func(*cid.Cid) *filestore.ListRes) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for _, arg := range args {
			c, err := cid.Decode(arg)
			if err != nil {
				out <- &filestore.ListRes{
					Status:   filestore.StatusOtherError,
					ErrorMsg: fmt.Sprintf("%s: %v", arg, err),
				}
				continue
			}
			r := action(c)
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
