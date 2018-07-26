package commands

import (
	"fmt"
	"os"
	"sort"

	oldCmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	"github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/filestore"

	cmds "gx/ipfs/QmNueRyPRQiV7PUEpnP4GgGLuK1rKQLaRW7sfPvUetYig1/go-ipfs-cmds"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     lsFileStore,
		"verify": verifyFileStore,
		"dups":   lgc.NewCommand(dupsFileStore),
	},
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		listOrVerify(req, re, env, false)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(req *cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
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

var verifyFileStore = &cmds.Command{
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		listOrVerify(req, re, env, true)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(req *cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
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

					if r.Status == filestore.StatusOtherError {
						fmt.Fprintf(os.Stderr, "%s\n", r.ErrorMsg)
					}
					fmt.Fprintf(os.Stdout, "%s %s\n", r.Status.Format(), r.FormatLong())
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

func getFilestore(env interface{}) (*core.IpfsNode, *filestore.Filestore, error) {
	n, err := GetNode(env)
	if err != nil {
		return nil, nil, err
	}
	fs := n.Filestore
	if fs == nil {
		return n, nil, filestore.ErrFilestoreNotEnabled
	}
	return n, fs, err
}

func listOrVerify(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment, verify bool) {
	_, fs, err := getFilestore(env)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	single := filestore.List
	all := filestore.ListAll
	if verify {
		single = filestore.Verify
		all = filestore.VerifyAll
	}

	if len(req.Arguments) > 0 {
		cids := make([]*cid.Cid, len(req.Arguments))
		var err error
		for i, cs := range req.Arguments {
			cids[i], err = cid.Decode(cs)
			if err != nil {
				re.SetError(fmt.Errorf("%s is not a valid cid: %s", cs, err), cmdkit.ErrClient)
				return
			}
		}
		for _, c := range cids {
			if err := re.Emit(single(fs, c)); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			if err := req.Context.Err(); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		}
		return
	}
	fileOrder, _ := req.Options["file-order"].(bool)
	ch, err := all(req.Context, fs)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
	if fileOrder {
		var results []*filestore.ListRes
		for r := range ch {
			results = append(results, r)
		}
		// TODO: Should be handled by the cmdkit library.
		if err := req.Context.Err(); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].FilePath == results[j].FilePath {
				// XXX: is this case even possible?
				if results[i].Offset == results[j].Offset {
					return results[i].Key.KeyString() < results[j].Key.KeyString()
				}
				return results[i].Offset < results[j].Offset
			}
			return results[i].FilePath < results[j].FilePath
		})
		for _, r := range results {
			re.Emit(r)
		}
		return
	}

	for r := range ch {
		if err := re.Emit(r); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	}
	// TODO: Should be handled by the cmdkit library.
	if err := req.Context.Err(); err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
}
