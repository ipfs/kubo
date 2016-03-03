package commands

import (
	"bytes"
	"fmt"
	"io"

	humanize "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"
	"github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/blocks/set"
	cmds "github.com/ipfs/go-ipfs/commands"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	"github.com/ipfs/go-ipfs/unixfs"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo.",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gc":       repoGcCmd,
		"ls-roots": repoLsRootsCmd,
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

type RepoLsRootsOutput struct {
	Hash   string
	Size   uint64
	Type   string
	Pinned string
}

var repoLsRootsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Display the top nodes stored locally",
		ShortDescription: `
'ipfs repo ls-roots' will display the top-level files or directory
that are stored locally as well a some of their properties.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Force pinner flush as we ask the blockstore directly
		n.Pinning.Flush()

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		keychan, err := n.Blockstore.AllKeysChan(req.Context())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		go func() {
			defer close(outChan)
			roots := set.NewSimpleBlockSet()
			childs := set.NewSimpleBlockSet()

			// find the roots of the DAG
		KeyLoop:
			for k := range keychan {

				// skip internal pinning node
				for _, pinnedKey := range n.Pinning.InternalPins() {
					if pinnedKey == k {
						continue KeyLoop
					}
				}

				node, err := n.DAG.Get(req.Context(), k)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				if !childs.HasKey(k) {
					roots.AddBlock(k)
				}

				for _, child := range node.Links {
					childKey := key.Key(child.Hash)
					roots.RemoveBlock(childKey)
					childs.AddBlock(childKey)
				}
			}

			for _, k := range roots.GetKeys() {
				node, err := n.DAG.Get(req.Context(), k)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				unixFSNode, err := unixfs.FromBytes(node.Data)
				if err != nil {
					// ignore nodes that are not unixFs
					continue
				}

				pinned, _, err := n.Pinning.IsPinned(k)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				size, err := node.Size()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				outChan <- &RepoLsRootsOutput{
					Hash:   k.B58String(),
					Size:   size,
					Type:   unixFSNode.Type.String(),
					Pinned: pinned,
				}
			}
		}()
	},

	Type: RepoLsRootsOutput{},

	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*RepoLsRootsOutput)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)

				fmt.Fprintf(buf, "%s\t%s\t%s    \t%s\n",
					obj.Hash,
					humanize.Bytes(obj.Size),
					obj.Type,
					obj.Pinned)

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
