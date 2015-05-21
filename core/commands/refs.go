package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
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

		objs, err := objectsForPaths(ctx, n, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		out := make(chan interface{})
		res.SetOutput((<-chan interface{})(out))

		go func() {
			defer close(out)

			rw := RefWriter{
				out:       out,
				DAG:       n.DAG,
				Ctx:       ctx,
				Unique:    unique,
				PrintEdge: edges,
				PrintFmt:  format,
				Recursive: recursive,
			}

			for _, o := range objs {
				if _, err := rw.WriteRefs(o); err != nil {
					out <- &RefWrapper{Err: err.Error()}
					return
				}
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*RefWrapper)
				if !ok {
					fmt.Println("%#v", v)
					return nil, u.ErrCast()
				}

				if obj.Err != "" {
					return nil, errors.New(obj.Err)
				}

				return strings.NewReader(obj.Ref), nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
			}, nil
		},
	},
	Type: RefWrapper{},
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

		go func() {
			defer pipew.Close()

			for k := range allKeys {
				s := k.Pretty() + "\n"
				if _, err := pipew.Write([]byte(s)); err != nil {
					log.Error("pipe write error: ", err)
					return
				}
			}
		}()

		res.SetOutput(piper)
	},
}

func objectsForPaths(ctx context.Context, n *core.IpfsNode, paths []string) ([]*dag.Node, error) {
	objects := make([]*dag.Node, len(paths))
	for i, p := range paths {
		o, err := core.Resolve(ctx, n, path.Path(p))
		if err != nil {
			return nil, err
		}
		objects[i] = o
	}
	return objects, nil
}

type RefWrapper struct {
	Ref string
	Err string
}

type RefWriter struct {
	out chan interface{}
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
	if rw.Recursive {
		return rw.writeRefsRecursive(n)
	} else {
		return rw.writeRefsSingle(n)
	}
}

func (rw *RefWriter) writeRefsRecursive(n *dag.Node) (int, error) {
	nkey, err := n.Key()
	if err != nil {
		return 0, err
	}

	if rw.skip(nkey) {
		return 0, nil
	}

	var count int
	for i, ng := range rw.DAG.GetDAG(rw.Ctx, n) {
		lk := u.Key(n.Links[i].Hash)
		if rw.skip(lk) {
			continue
		}

		if err := rw.WriteEdge(nkey, lk, n.Links[i].Name); err != nil {
			return count, err
		}

		nd, err := ng.Get(rw.Ctx)
		if err != nil {
			return count, err
		}

		c, err := rw.writeRefsRecursive(nd)
		count += c
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (rw *RefWriter) writeRefsSingle(n *dag.Node) (int, error) {
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

	rw.out <- &RefWrapper{Ref: s}
	return nil
}
