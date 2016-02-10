package commands

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	humanize "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

type RepoStat struct {
	repoPath  string
	repoSize  uint64 // size in bytes
	numBlocks uint64
}

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo.",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gc":   repoGcCmd,
		"stat": repoStatCmd,
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

var repoStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Print status of the local repo.",
		ShortDescription: ``,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		usage, err := n.Repo.GetStorageUsage()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		allKeys, err := n.Blockstore.AllKeysChan(ctx)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		count := uint64(0)
		for range allKeys {
			count++
		}

		path, err := fsrepo.BestKnownPath()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&RepoStat{
			repoPath:  path,
			repoSize:  usage,
			numBlocks: count,
		})
	},
	Type: RepoStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			stat, ok := res.Output().(*RepoStat)
			if !ok {
				return nil, u.ErrCast()
			}

			out := fmt.Sprintf(
				"Path: %s\nSize: %s\nBlocks: %d\n",
				stat.repoPath, humanize.Bytes(stat.repoSize), stat.numBlocks)

			return strings.NewReader(out), nil
		},
	},
}
