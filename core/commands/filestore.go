package commands

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	//ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":         lsFileStore,
		"verify":     verifyFileStore,
		"rm":         rmFilestoreObjs,
		"rm-invalid": rmInvalidObjs,
		//"rm-incomplete":      rmIncompleteObjs,
		"find-dangling-pins": findDanglingPins,
	},
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore",
		ShortDescription: `
List objects in the filestore.  If --quiet is specified only the
hashes are printed, otherwise the fields are as follows:
  <hash> <type> <filepath> <offset> <size> <modtime>
where <type> is one of"
  leaf: to indicate a leaf node where the contents are stored
        to in the file itself
  root: to indicate a root node that represents the whole file
  other: some other kind of node that represent part of a file
  invld: a leaf node that has been found invalid
and <filepath> is the part of the file the object represents.  The
part represented starts at <offset> and continues for <size> bytes.
If <offset> is the special value "-" than the "leaf" or "root" node
represents the whole file.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write just hashes of objects."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := res.Request().Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, _ := filestore.List(fs, quiet)
		res.SetOutput(&chanWriter{ch, "", 0})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

type chanWriter struct {
	ch     <-chan filestore.ListRes
	buf    string
	offset int
}

func (w *chanWriter) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch
		if !more {
			return 0, io.EOF
		}
		if res.DataObj == nil {
			w.buf = res.MHash() + "\n"
		} else {
			w.buf = res.Format()
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore",
		ShortDescription: `
Verify leaf nodes in the filestore, the output is:
  <status> <type> <filepath> <offset> <size> <modtime>
where <type>, <filepath>, <offset> and <size> are the same as in the
"ls" command and <status> is one of:
  ok:      If the object is okay
  changed: If the object is invalid becuase the contents of the file
           have changed
  missing: If the file can not be found
  error:   If the file can be found but could not be read or some
           other error
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, _ := filestore.List(fs, false)
		rdr, wtr := io.Pipe()
		go func() {
			defer wtr.Close()
			for res := range ch {
				if !res.NoBlockData() {
					continue
				}
				res.Status = filestore.Verify(fs, res.Key, res.DataObj)
				wtr.Write([]byte(res.Format()))
			}
		}()
		res.SetOutput(rdr)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmFilestoreObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove objects from the filestore",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Multi-hashes to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		_ = fs
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := res.Request().Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		rdr, wtr := io.Pipe()
		var rmWtr io.Writer = wtr
		if quiet {
			rmWtr = ioutil.Discard
		}
		go func() {
			numErrors := 0
			for _, mhash := range hashes {
				key := k.B58KeyDecode(mhash)
				err = delFilestoreObj(req, rmWtr, node, fs, key)
				if err != nil {
					fmt.Fprintf(wtr, "Error deleting %s: %s\n", mhash, err.Error())
					numErrors += 1
				}
			}
			if numErrors > 0 {
				wtr.CloseWithError(errors.New("Could not delete some keys."))
				return
			}
			wtr.Close()
		}()
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmInvalidObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove invalid objects from the filestore",
		ShortDescription: `
Removes objects that have become invalid from the Filestrore up to the
reason specified in <level>.  If <level> is "changed" than remove any
blocks that have become invalid due to the contents of the underlying
file changing.  If <level> is "missing" also remove any blocks that
have become invalid because the underlying file is no longer available
due to a "No such file" or related error, but not if the file exists
but is unreadable for some reason.  If <level> is "all" remove any
blocks that fail to validate regardless of the reason.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("level", true, false, "one of changed, missing. or all").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
		cmds.BoolOption("dry-run", "n", "Do everything except the actual delete."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		_ = fs
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		args := req.Arguments()
		if len(args) != 1 {
			res.SetError(errors.New("invalid usage"), cmds.ErrNormal)
			return
		}
		mode := req.Arguments()[0]
		level := filestore.StatusMissing
		switch mode {
		case "changed":
			level = filestore.StatusChanged
		case "missing":
			level = filestore.StatusMissing
		case "all":
			level = filestore.StatusError
		default:
			res.SetError(errors.New("level must be one of: changed missing all"), cmds.ErrNormal)
		}
		quiet, _, err := res.Request().Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		dryRun, _, err := res.Request().Option("dry-run").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, _ := filestore.List(fs, false)
		rdr, wtr := io.Pipe()
		var rmWtr io.Writer = wtr
		if quiet {
			rmWtr = ioutil.Discard
		}
		go func() {
			var toDel [][]byte
			for r := range ch {
				if !r.NoBlockData() {
					continue
				}
				r.Status = filestore.Verify(fs, r.Key, r.DataObj)
				if r.Status >= level {
					toDel = append(toDel, r.RawHash())
					if !quiet {
						fmt.Fprintf(wtr, "will delete %s (part of %s)\n", r.MHash(), r.FilePath)
					}
				}
			}
			if dryRun {
				fmt.Fprintf(wtr, "Dry-run option specified.  Stopping.\n")
				fmt.Fprintf(wtr, "Would of deleted %d invalid objects.\n", len(toDel))
			} else {
				for _, key := range toDel {
					err = delFilestoreObj(req, rmWtr, node, fs, k.Key(key))
					if err != nil {
						mhash := b58.Encode(key)
						msg := fmt.Sprintf("Could not delete %s: %s\n", mhash, err.Error())
						wtr.CloseWithError(errors.New(msg))
						return

					}
				}
				fmt.Fprintf(wtr, "Deleted %d invalid objects.\n", len(toDel))
			}
			wtr.Close()
		}()
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func delFilestoreObj(req cmds.Request, out io.Writer, node *core.IpfsNode, fs *filestore.Datastore, key k.Key) error {
	err := fs.DeleteDirect(key.DsKey())
	if err != nil {
		return err
	}
	stillExists, err := node.Blockstore.Has(key)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted %s\n", key)
	if stillExists {
		return nil
	}
	_, pinned1, err := node.Pinning.IsPinnedWithType(key, "recursive")
	if err != nil {
		return err
	}
	_, pinned2, err := node.Pinning.IsPinnedWithType(key, "direct")
	if err != nil {
		return err
	}
	if pinned1 || pinned2 {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		err = node.Pinning.Unpin(ctx, key, true)
		if err != nil {
			return err
		}
		err := node.Pinning.Flush()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Unpinned %s\n", key)
	}
	return nil
}

func extractFilestore(req cmds.Request) (node *core.IpfsNode, fs *filestore.Datastore, err error) {
	node, err = req.InvocContext().GetNode()
	if err != nil {
		return
	}
	repo, ok := node.Repo.Self().(*fsrepo.FSRepo)
	if !ok {
		err = errors.New("Not a FSRepo")
		return
	}
	fs = repo.Filestore()
	return
}

var findDanglingPins = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List pinned objects that no longer exists",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			err := listDanglingPins(n.Pinning.DirectKeys(), w, n.Blockstore)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			err = listDanglingPins(n.Pinning.RecursiveKeys(), w, n.Blockstore)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func listDanglingPins(keys []k.Key, out io.Writer, d bs.Blockstore) error {
	for _, k := range keys {
		exists, err := d.Has(k)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Fprintln(out, k.B58String())
		}
	}
	return nil
}
