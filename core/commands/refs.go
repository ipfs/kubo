package commands

import (
	"bytes"
	"io"
	"strings"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	path "github.com/jbenet/go-ipfs/path"
	u "github.com/jbenet/go-ipfs/util"
)

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []u.Key
}

// KeyListTextMarshaler outputs a KeyList as plaintext, one key per line
func KeyListTextMarshaler(res cmds.Response) (io.Reader, error) {
	output := res.Output().(*KeyList)
	var buf bytes.Buffer
	for _, key := range output.Keys {
		buf.WriteString(key.B58String() + "\n")
	}
	return &buf, nil
}

var RefsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Lists links (references) from an object",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and displays the link
hashes it contains, with the following format:

  <link base58 hash>

Note: list all refs recursively with -r.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"local": RefsLocalCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to the object(s) to list refs from").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "Emit edges with given format. tokens: <src> <dst> <linkname>"),
		cmds.BoolOption("edges", "e", "Emit edge format: `<from> -> <to>`"),
		cmds.BoolOption("unique", "u", "Omit duplicate refs from output"),
		cmds.BoolOption("recursive", "r", "Recursively list links of child nodes"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context().Context
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		unique, _, err := req.Option("unique").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		recursive, _, err := req.Option("recursive").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		edges, _, err := req.Option("edges").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		format, _, err := req.Option("format").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		objs, err := objectsForPaths(n, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		piper, pipew := io.Pipe()
		eptr := &ErrPassThroughReader{R: piper}

		go func() {
			defer pipew.Close()

			rw := RefWriter{
				W:         pipew,
				DAG:       n.DAG,
				Ctx:       ctx,
				Unique:    unique,
				PrintEdge: edges,
				PrintFmt:  format,
				Recursive: recursive,
			}

			for _, o := range objs {
				if _, err := rw.WriteRefs(o); err != nil {
					eptr.SetError(err)
					return
				}
			}
		}()

		res.SetOutput(eptr)
	},
}

var RefsLocalCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Lists all local references",
		ShortDescription: `
Displays the hashes of all local objects.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context().Context
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// todo: make async
		allKeys, err := n.Blockstore.AllKeysChan(ctx)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		piper, pipew := io.Pipe()
		eptr := &ErrPassThroughReader{R: piper}

		go func() {
			defer pipew.Close()

			for k := range allKeys {
				s := k.Pretty() + "\n"
				if _, err := pipew.Write([]byte(s)); err != nil {
					eptr.SetError(err)
					return
				}
			}
		}()

		res.SetOutput(eptr)
	},
}

func objectsForPaths(n *core.IpfsNode, paths []string) ([]*dag.Node, error) {
	objects := make([]*dag.Node, len(paths))
	for i, p := range paths {
		o, err := n.Resolver.ResolvePath(path.Path(p))
		if err != nil {
			return nil, err
		}
		objects[i] = o
	}
	return objects, nil
}

// ErrPassThroughReader is a reader that may return an externally set error.
type ErrPassThroughReader struct {
	R   io.ReadCloser
	err error

	sync.RWMutex
}

func (r *ErrPassThroughReader) Error() error {
	r.RLock()
	defer r.RUnlock()
	return r.err
}

func (r *ErrPassThroughReader) SetError(err error) {
	r.Lock()
	r.err = err
	r.Unlock()
}

func (r *ErrPassThroughReader) Read(buf []byte) (int, error) {
	err := r.Error()
	if err != nil {
		return 0, err
	}

	return r.R.Read(buf)
}

func (r *ErrPassThroughReader) Close() error {
	err1 := r.R.Close()
	err2 := r.Error()
	if err2 != nil {
		return err2
	}
	return err1
}

type RefWriter struct {
	W   io.Writer
	DAG dag.DAGService
	Ctx context.Context

	Unique    bool
	Recursive bool
	PrintEdge bool
	PrintFmt  string

	seen map[u.Key]struct{}
}

// WriteRefs writes refs of the given object to the underlying writer.
func (rw *RefWriter) WriteRefs(n *dag.Node) (int, error) {
	nkey, err := n.Key()
	if err != nil {
		return 0, err
	}

	if rw.skip(nkey) {
		return 0, nil
	}

	count := 0
	for _, l := range n.Links {
		lk := u.Key(l.Hash)

		if rw.skip(lk) {
			continue
		}

		if err := rw.WriteEdge(nkey, lk, l.Name); err != nil {
			return count, err
		}
		count++

		if !rw.Recursive {
			continue
		}

		child, err := l.GetNode(rw.DAG)
		if err != nil {
			return count, err
		}

		c, err := rw.WriteRefs(child)
		count += c
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

// skip returns whether to skip a key
func (rw *RefWriter) skip(k u.Key) bool {
	if !rw.Unique {
		return false
	}

	if rw.seen == nil {
		rw.seen = make(map[u.Key]struct{})
	}

	_, found := rw.seen[k]
	if !found {
		rw.seen[k] = struct{}{}
	}
	return found
}

// Write one edge
func (rw *RefWriter) WriteEdge(from, to u.Key, linkname string) error {
	if rw.Ctx != nil {
		select {
		case <-rw.Ctx.Done(): // just in case.
			return rw.Ctx.Err()
		default:
		}
	}

	var s string
	switch {
	case rw.PrintFmt != "":
		s = rw.PrintFmt
		s = strings.Replace(s, "<src>", from.Pretty(), -1)
		s = strings.Replace(s, "<dst>", to.Pretty(), -1)
		s = strings.Replace(s, "<linkname>", linkname, -1)
	case rw.PrintEdge:
		s = from.Pretty() + " -> " + to.Pretty()
	default:
		s += to.Pretty()
	}
	s += "\n"

	if _, err := rw.W.Write([]byte(s)); err != nil {
		return err
	}
	return nil
}
