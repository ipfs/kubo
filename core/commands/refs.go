package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []key.Key
}

// KeyListTextMarshaler outputs a KeyList as plaintext, one key per line
func KeyListTextMarshaler(res cmds.Response) (io.Reader, error) {
	output := res.Output().(*KeyList)
	buf := new(bytes.Buffer)
	for _, key := range output.Keys {
		buf.WriteString(key.B58String() + "\n")
	}
	return buf, nil
}

var RefsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Lists links (references) from an object.",
		ShortDescription: `
Lists the hashes of all the links an IPFS or IPNS object(s) contains,
with the following format:

  <link base58 hash>

NOTE: List all references recursively by using the flag '-r'.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"local": RefsLocalCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to the object(s) to list refs from.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "Emit edges with given format. Available tokens: <src> <dst> <linkname>.").Default("<dst>"),
		cmds.BoolOption("edges", "e", "Emit edge format: `<from> -> <to>`.").Default(false),
		cmds.BoolOption("unique", "u", "Omit duplicate refs from output.").Default(false),
		cmds.BoolOption("recursive", "r", "Recursively list links of child nodes.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
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

				return strings.NewReader(obj.Ref + "\n"), nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: RefWrapper{},
}

var RefsLocalCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Lists all local references.",
		ShortDescription: `
Displays the hashes of all local objects.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
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
				s := k.B58String() + "\n"
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

	seen map[key.Key]struct{}
}

// WriteRefs writes refs of the given object to the underlying writer.
func (rw *RefWriter) WriteRefs(n *dag.Node) (int, error) {
	if rw.Recursive {
		return rw.writeRefsRecursive(n)
	}
	return rw.writeRefsSingle(n)
}

func (rw *RefWriter) writeRefsRecursive(n *dag.Node) (int, error) {
	nkey, err := n.Key()
	if err != nil {
		return 0, err
	}

	var count int
	for i, ng := range dag.GetDAG(rw.Ctx, rw.DAG, n) {
		lk := key.Key(n.Links[i].Hash)
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
		lk := key.Key(l.Hash)

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
func (rw *RefWriter) skip(k key.Key) bool {
	if !rw.Unique {
		return false
	}

	if rw.seen == nil {
		rw.seen = make(map[key.Key]struct{})
	}

	_, found := rw.seen[k]
	if !found {
		rw.seen[k] = struct{}{}
	}
	return found
}

// Write one edge
func (rw *RefWriter) WriteEdge(from, to key.Key, linkname string) error {
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
		s = strings.Replace(s, "<src>", from.B58String(), -1)
		s = strings.Replace(s, "<dst>", to.B58String(), -1)
		s = strings.Replace(s, "<linkname>", linkname, -1)
	case rw.PrintEdge:
		s = from.B58String() + " -> " + to.B58String()
	default:
		s += to.B58String()
	}

	rw.out <- &RefWrapper{Ref: s}
	return nil
}
