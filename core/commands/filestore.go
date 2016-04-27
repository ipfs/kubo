package commands

import (
	"errors"
	"fmt"
	"io"

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

type chanWriter struct {
	ch       <-chan *filestore.ListRes
	buf      string
	offset   int
	hashOnly bool
}

func (w *chanWriter) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch
		if !more {
			return 0, io.EOF
		}
		if w.hashOnly {
			w.buf = b58.Encode(res.Key) + "\n"
		} else {
			w.buf = res.Format()
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":                 lsFileStore,
		"verify":             verifyFileStore,
		"rm":                 rmFilestoreObjs,
		"find-dangling-pins": findDanglingPins,
	},
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore",
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
		ch := make(chan *filestore.ListRes)
		go func() {
			defer close(ch)
			filestore.List(fs, ch)
		}()
		res.SetOutput(&chanWriter{ch, "", 0, quiet})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch := make(chan *filestore.ListRes)
		go func() {
			defer close(ch)
			filestore.Verify(fs, ch)
		}()
		res.SetOutput(&chanWriter{ch, "", 0, false})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmFilestoreObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove objects from the Filestore",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Multi-hashes to remove.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		_ = fs
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		serr := res.Stderr()
		numErrors := 0
		for _, mhash := range hashes {
			err = delFilestoreObj(req, node, fs, mhash)
			if err != nil {
				fmt.Fprintf(serr, "Error deleting %s: %s\n", mhash, err.Error())
				numErrors += 1
			}
		}
		if numErrors > 0 {
			res.SetError(errors.New("Could not delete some keys"), cmds.ErrNormal)
			return
		}
		return
	},
}

func delFilestoreObj(req cmds.Request, node *core.IpfsNode, fs *filestore.Datastore, mhash string) error {
	key := k.B58KeyDecode(mhash)
	err := fs.DeleteDirect(key.DsKey())
	if err != nil {
		return err
	}
	stillExists, err := node.Blockstore.Has(key)
	if err != nil {
		return err
	}
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
		println("unpinning")
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
