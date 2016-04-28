package commands

import (
	"errors"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

type chanWriter struct {
	ch     <-chan *filestore.ListRes
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
		w.buf = res.Format()
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
		"ls":     lsFileStore,
		"verify": verifyFileStore,
	},
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects on filestore",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		fsrepo, ok := node.Repo.Self().(*fsrepo.FSRepo)
		if !ok {
			res.SetError(errors.New("Not a FSRepo"), cmds.ErrNormal)
			return
		}
		ch := make(chan *filestore.ListRes)
		go func() {
			defer close(ch)
			filestore.List(fsrepo.Filestore(), ch)
		}()
		res.SetOutput(&chanWriter{ch, "", 0})
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
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		fsrepo, ok := node.Repo.Self().(*fsrepo.FSRepo)
		if !ok {
			res.SetError(errors.New("Not a FSRepo"), cmds.ErrNormal)
			return
		}
		ch := make(chan *filestore.ListRes)
		go func() {
			defer close(ch)
			filestore.Verify(fsrepo.Filestore(), ch)
		}()
		res.SetOutput(&chanWriter{ch, "", 0})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}
