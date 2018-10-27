package commands

import (
	"context"
	"fmt"
	"io"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	filestore "github.com/ipfs/go-ipfs/filestore"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     lsFileStore,
		"verify": verifyFileStore,
		"dups":   dupsFileStore,
	},
}

const (
	fileOrderOptionName = "file-order"
)

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
		cmdkit.BoolOption(fileOrderOptionName, "sort the results based on the path of the backing file"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}
		args := req.Arguments
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context, args, func(c cid.Cid) *filestore.ListRes {
				return filestore.List(fs, c)
			})

			return res.Emit(out)
		}

		fileOrder, _ := req.Options[fileOrderOptionName].(bool)
		next, err := filestore.ListAll(fs, fileOrder)
		if err != nil {
			return err
		}

		out := listResToChan(req.Context, next)
		return res.Emit(out)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: streamResult(func(v interface{}, out io.Writer) nonFatalError {
			r := v.(*filestore.ListRes)
			if r.ErrorMsg != "" {
				return nonFatalError(r.ErrorMsg)
			}
			fmt.Fprintf(out, "%s\n", r.FormatLong())
			return ""
		}),
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
		cmdkit.BoolOption(fileOrderOptionName, "verify the objects based on the order of the backing file"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}
		args := req.Arguments
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context, args, func(c cid.Cid) *filestore.ListRes {
				return filestore.Verify(fs, c)
			})
			return res.Emit(out)
		} else {
			fileOrder, _ := req.Options[fileOrderOptionName].(bool)
			next, err := filestore.VerifyAll(fs, fileOrder)
			if err != nil {
				return err
			}
			out := listResToChan(req.Context, next)
			return res.Emit(out)
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *filestore.ListRes) error {
			if out.Status == filestore.StatusOtherError {
				return fmt.Errorf(out.ErrorMsg)
			}
			fmt.Fprintf(w, "%s %s\n", out.Status.Format(), out.FormatLong())
			return nil
		}),
	},
	Type: filestore.ListRes{},
}

var dupsFileStore = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List blocks that are both in the filestore and standard block storage.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}
		ch, err := fs.FileManager().AllKeysChan(req.Context)
		if err != nil {
			return err
		}

		out := make(chan interface{}, 128)

		go func() {
			defer close(out)
			for cid := range ch {
				have, err := fs.MainBlockstore().Has(cid)
				if err != nil {
					select {
					case out <- &RefWrapper{Err: err.Error()}:
					case <-req.Context.Done():
					}
					return
				}
				if have {
					select {
					case out <- &RefWrapper{Ref: cid.String()}:
					case <-req.Context.Done():
						return
					}
				}
			}
		}()
		return res.Emit(out)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RefWrapper) error {
			if out.Err != "" {
				return fmt.Errorf(out.Err)
			}

			fmt.Fprintln(w, out.Ref)

			return nil
		}),
	},
	Type: RefWrapper{},
}

func getFilestore(env cmds.Environment) (*core.IpfsNode, *filestore.Filestore, error) {
	n, err := cmdenv.GetNode(env)
	if err != nil {
		return nil, nil, err
	}
	fs := n.Filestore
	if fs == nil {
		return n, nil, filestore.ErrFilestoreNotEnabled
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

func perKeyActionToChan(ctx context.Context, args []string, action func(cid.Cid) *filestore.ListRes) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for _, arg := range args {
			c, err := cid.Decode(arg)
			if err != nil {
				select {
				case out <- &filestore.ListRes{
					Status:   filestore.StatusOtherError,
					ErrorMsg: fmt.Sprintf("%s: %v", arg, err),
				}:
				case <-ctx.Done():
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
