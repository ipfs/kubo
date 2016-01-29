package commands

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	u "github.com/ipfs/go-ipfs/util"
	"github.com/ipfs/go-ipfs/unixfs"
)

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo.",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gc": repoGcCmd,
		"ls": repoLsCmd,
	},
}

var repoGcCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Perform a garbage collection sweep on the repo.",
		ShortDescription: `
'ipfs repo gc' is a plumbing command that will sweep the local
set of stored objects and remove ones that are not pinned in
order to reclaim hard disk space.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write minimal output."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		gcOutChan, err := corerepo.GarbageCollectAsync(n, req.Context())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for k := range gcOutChan {
				outChan <- k
			}
		}()
	},
	Type: corerepo.KeyRemoved{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			quiet, _, err := res.Request().Option("quiet").Bool()
			if err != nil {
				return nil, err
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*corerepo.KeyRemoved)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				if quiet {
					buf = bytes.NewBufferString(string(obj.Key) + "\n")
				} else {
					buf = bytes.NewBufferString(fmt.Sprintf("removed %s\n", obj.Key))
				}
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
}

type RepoLsOutput struct {
	Hash string
	Size uint64
	Type string
}

var repoLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Totally need a documentation",
		ShortDescription: `
		I might agree on that
		`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for _, k := range n.Pinning.RecursiveKeys() {
				node, err := n.DAG.Get(req.Context(), k)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				unixFSNode, err := unixfs.FromBytes(node.Data);
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				outChan <- &RepoLsOutput{
					Hash: k.Pretty(),
					Size: unixFSNode.GetFilesize(),
					Type: unixFSNode.GetType().String(),
				}
			}
		}()
	},

	Type: RepoLsOutput{},

	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*RepoLsOutput)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)

				fmt.Fprintf(buf, "%s\t%s     \t%v\n", obj.Hash, obj.Type, obj.Size)

				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
}
