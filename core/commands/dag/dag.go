package dagcmd

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	coredag "github.com/ipfs/go-ipfs/core/coredag"
	pin "github.com/ipfs/go-ipfs/pin"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"

	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	files "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit/files"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

var DagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid *cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     *cid.Cid
	RemPath string
}

var DagPutCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("format", "f", "Format that the object will be added as.").WithDefault("cbor"),
		cmdkit.StringOption("input-enc", "Format that the input object will be.").WithDefault("json"),
		cmdkit.BoolOption("pin", "Pin this object when adding."),
		cmdkit.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		ienc, _, _ := req.Option("input-enc").String()
		format, _, _ := req.Option("format").String()
		hash, _, err := req.Option("hash").String()
		dopin, _, err := req.Option("pin").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// mhType tells inputParser which hash should be used. MaxUint64 means 'use
		// default hash' (sha256 for cbor, sha1 for git..)
		mhType := uint64(math.MaxUint64)

		if hash != "" {
			var ok bool
			mhType, ok = mh.Names[hash]
			if !ok {
				res.SetError(fmt.Errorf("%s in not a valid multihash name", hash), cmdkit.ErrNormal)

				return
			}
		}

		outChan := make(chan interface{}, 8)
		res.SetOutput((<-chan interface{})(outChan))

		addAllAndPin := func(f files.File) error {
			cids := cid.NewSet()
			b := ipld.NewBatch(req.Context(), n.DAG)

			for {
				file, err := f.NextFile()
				if err == io.EOF {
					// Finished the list of files.
					break
				} else if err != nil {
					return err
				}

				nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
				if err != nil {
					return err
				}
				if len(nds) == 0 {
					return fmt.Errorf("no node returned from ParseInputs")
				}

				for _, nd := range nds {
					err := b.Add(nd)
					if err != nil {
						return err
					}
				}

				cid := nds[0].Cid()
				cids.Add(cid)

				select {
				case outChan <- &OutputObject{Cid: cid}:
				case <-req.Context().Done():
					return nil
				}
			}

			if err := b.Commit(); err != nil {
				return err
			}

			if dopin {
				defer n.Blockstore.PinLock().Unlock()

				cids.ForEach(func(c *cid.Cid) error {
					n.Pinning.PinWithMode(c, pin.Recursive)
					return nil
				})

				err := n.Pinning.Flush()
				if err != nil {
					return err
				}
			}

			return nil
		}

		go func() {
			defer close(outChan)
			if err := addAllAndPin(req.Files()); err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}()
	},
	Type: OutputObject{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			oobj, ok := v.(*OutputObject)
			if !ok {
				return nil, e.TypeErr(oobj, v)
			}

			return strings.NewReader(oobj.Cid.String() + "\n"), nil
		},
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specified
format.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		lastCid, rem, err := n.Resolver.ResolveToLastNode(req.Context(), p)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		obj, err := n.DAG.Get(req.Context(), lastCid)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var out interface{} = obj
		if len(rem) > 0 {
			final, _, err := obj.Resolve(rem)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			out = final
		}

		res.SetOutput(out)
	},
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Resolve ipld block",
		ShortDescription: `
'ipfs dag resolve' fetches a dag node from ipfs, prints it's address and remaining path.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		lastCid, rem, err := n.Resolver.ResolveToLastNode(req.Context(), p)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&ResolveOutput{
			Cid:     lastCid,
			RemPath: path.Join(rem),
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output := v.(*ResolveOutput)
			buf := new(bytes.Buffer)
			p := output.Cid.String()
			if output.RemPath != "" {
				p = path.Join([]string{p, output.RemPath})
			}

			buf.WriteString(p)

			return buf, nil
		},
	},
	Type: ResolveOutput{},
}

// copy+pasted from ../commands.go
func unwrapOutput(i interface{}) (interface{}, error) {
	var (
		ch <-chan interface{}
		ok bool
	)

	if ch, ok = i.(<-chan interface{}); !ok {
		return nil, e.TypeErr(ch, i)
	}

	return <-ch, nil
}
