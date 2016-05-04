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
	fsutil "github.com/ipfs/go-ipfs/filestore/util"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
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
  <hash> <type> <filepath> <offset> <size> [<modtime>]
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
		ch, _ := fsutil.List(fs, quiet)
		res.SetOutput(&chanWriter{ch, "", 0, false})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

type chanWriter struct {
	ch     <-chan fsutil.ListRes
	buf    string
	offset int
	errors bool
}

func (w *chanWriter) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch
		if !more && !w.errors {
			return 0, io.EOF
		} else if !more && w.errors {
			return 0, errors.New("Some checks failed.")
		} else if fsutil.AnError(res.Status) {
			w.errors = true
		}
		w.buf = res.Format()
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore",
		ShortDescription: `
Verify nodes in the filestore.  The output is:
  <status> [<type> <filepath> <offset> <size> [<modtime>]]
where <type>, <filepath>, <offset>, <size> and <modtime> are the same
as in the "ls".  <status> is one of
  ok:      If the original data can be reconstructed
  complete: If all the blocks in the tree exists but no attempt was
            made to reconstruct the original data

  incomplete: Some of the blocks of the tree could not be read

  changed: If the leaf node is invalid because the contents of the file
           have changed
  no-file: If the file can not be found
  error:   If the file can be found but could not be read or some
           other error

  ERROR:   The block could not be read due to an internal error

  found:   The child of another node was found outside the filestore
  missing: The child of another node does not exist
  <blank>: The child of another node node exists but no attempt was
           made to verify it

  appended: The node is still valid but the original file was appended

  orphan: This node is a child of another node that was not found in
          the filestore
 
If --basic is specified then just scan leaf nodes to verify that they
are still valid.  Otherwise attempt to reconstruct the contents of of
all nodes and also check for orphan nodes (unless --skip-orphans is
also specified).

The --level option specifies how thorough the checks should be.  A
current meaning of the levels are:
  7-9: always check the contents
  2-6: check the contents if the modification time differs
  0-1: only check for the existence of blocks without verifying the
       contents of leaf nodes

The --verbose option specifies what to output.  The current values are:
  7-9: show everything
  5-6: don't show child nodes with a status of: ok, <blank>, or complete
  3-4: don't show child nodes
  0-2: don't child nodes and don't show root nodes with of: ok or complete
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("basic", "Perform a basic scan of leaf nodes only."),
		cmds.IntOption("level", "l", "0-9, Verification level.").Default(6),
		cmds.IntOption("verbose", "v", "0-9 Verbose level.").Default(6),
		cmds.BoolOption("skip-orphans", "Skip check for orphans."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		basic, _, err := res.Request().Option("basic").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		level, _, err := res.Request().Option("level").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		verbose, _, err := res.Request().Option("verbose").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if level < 0 || level > 9 {
			res.SetError(errors.New("level must be between 0-9"), cmds.ErrNormal)
			return
		}
		skipOrphans, _, err := res.Request().Option("skip-orphans").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if basic {
			ch, _ := fsutil.VerifyBasic(fs, level, verbose)
			res.SetOutput(&chanWriter{ch, "", 0, false})
		} else {
			ch, _ := fsutil.VerifyFull(node, fs, level, verbose, skipOrphans)
			res.SetOutput(&chanWriter{ch, "", 0, false})
		}
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
				err = fsutil.Delete(req, rmWtr, node, fs, key)
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
		rdr, err := fsutil.RmInvalid(req, node, fs, mode, quiet, dryRun)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
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
