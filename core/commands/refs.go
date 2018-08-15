package commands

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []*cid.Cid
}

// KeyListTextMarshaler outputs a KeyList as plaintext, one key per line
func KeyListTextMarshaler(res cmds.Response) (io.Reader, error) {
	out, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	output, ok := out.(*KeyList)
	if !ok {
		return nil, e.TypeErr(output, out)
	}

	buf := new(bytes.Buffer)
	for _, key := range output.Keys {
		buf.WriteString(key.String() + "\n")
	}
	return buf, nil
}

var RefsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List links (references) from an object.",
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
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "Path to the object(s) to list refs from.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "Emit edges with given format. Available tokens: <src> <dst> <linkname>.").WithDefault("<dst>"),
		cmdkit.BoolOption("edges", "e", "Emit edge format: `<from> -> <to>`."),
		cmdkit.BoolOption("unique", "u", "Omit duplicate refs from output."),
		cmdkit.BoolOption("recursive", "r", "Recursively list links of child nodes."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		unique, _, err := req.Option("unique").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		recursive, _, err := req.Option("recursive").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		format, _, err := req.Option("format").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		edges, _, err := req.Option("edges").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if edges {
			if format != "<dst>" {
				res.SetError(errors.New("using format argument with edges is not allowed"),
					cmdkit.ErrClient)
				return
			}

			format = "<src> -> <dst>"
		}

		objs, err := objectsForPaths(ctx, n, req.Arguments())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
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
				PrintFmt:  format,
				Recursive: recursive,
			}

			for _, o := range objs {
				if _, err := rw.WriteRefs(o); err != nil {
					select {
					case out <- &RefWrapper{Err: err.Error()}:
					case <-ctx.Done():
					}
					return
				}
			}
		}()
	},
	Marshalers: refsMarshallerMap,
	Type:       RefWrapper{},
}

var RefsLocalCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all local references.",
		ShortDescription: `
Displays the hashes of all local objects.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// todo: make async
		allKeys, err := n.Blockstore.AllKeysChan(ctx)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		out := make(chan interface{})
		res.SetOutput((<-chan interface{})(out))

		go func() {
			defer close(out)

			for k := range allKeys {
				select {
				case out <- &RefWrapper{Ref: k.String()}:
				case <-req.Context().Done():
					return
				}
			}
		}()
	},
	Marshalers: refsMarshallerMap,
	Type:       RefWrapper{},
}

var refsMarshallerMap = cmds.MarshalerMap{
	cmds.Text: func(res cmds.Response) (io.Reader, error) {
		v, err := unwrapOutput(res.Output())
		if err != nil {
			return nil, err
		}

		obj, ok := v.(*RefWrapper)
		if !ok {
			return nil, e.TypeErr(obj, v)
		}

		if obj.Err != "" {
			return nil, errors.New(obj.Err)
		}

		return strings.NewReader(obj.Ref + "\n"), nil
	},
}

func objectsForPaths(ctx context.Context, n *core.IpfsNode, paths []string) ([]ipld.Node, error) {
	objects := make([]ipld.Node, len(paths))
	for i, sp := range paths {
		p, err := path.ParsePath(sp)
		if err != nil {
			return nil, err
		}

		o, err := core.Resolve(ctx, n.Namesys, n.Resolver, p)
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
	DAG ipld.DAGService
	Ctx context.Context

	Unique    bool
	Recursive bool
	PrintFmt  string

	seen *cid.Set
}

// WriteRefs writes refs of the given object to the underlying writer.
func (rw *RefWriter) WriteRefs(n ipld.Node) (int, error) {
	if rw.Recursive {
		return rw.writeRefsRecursive(n)
	}
	return rw.writeRefsSingle(n)
}

func (rw *RefWriter) writeRefsRecursive(n ipld.Node) (int, error) {
	nc := n.Cid()

	var count int
	for i, ng := range ipld.GetDAG(rw.Ctx, rw.DAG, n) {
		lc := n.Links()[i].Cid
		if rw.skip(lc) {
			continue
		}

		if err := rw.WriteEdge(nc, lc, n.Links()[i].Name); err != nil {
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

func (rw *RefWriter) writeRefsSingle(n ipld.Node) (int, error) {
	c := n.Cid()

	if rw.skip(c) {
		return 0, nil
	}

	count := 0
	for _, l := range n.Links() {
		lc := l.Cid
		if rw.skip(lc) {
			continue
		}

		if err := rw.WriteEdge(c, lc, l.Name); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// skip returns whether to skip a cid
func (rw *RefWriter) skip(c *cid.Cid) bool {
	if !rw.Unique {
		return false
	}

	if rw.seen == nil {
		rw.seen = cid.NewSet()
	}

	has := rw.seen.Has(c)
	if !has {
		rw.seen.Add(c)
	}
	return has
}

// Write one edge
func (rw *RefWriter) WriteEdge(from, to *cid.Cid, linkname string) error {
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
		s = strings.Replace(s, "<src>", from.String(), -1)
		s = strings.Replace(s, "<dst>", to.String(), -1)
		s = strings.Replace(s, "<linkname>", linkname, -1)
	default:
		s += to.String()
	}

	rw.out <- &RefWrapper{Ref: s}
	return nil
}
