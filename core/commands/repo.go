package commands

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	corerepo "github.com/jbenet/go-ipfs/core/repo"
	u "github.com/jbenet/go-ipfs/util"
)

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gc": repoGcCmd,
	},
}

var repoGcCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Perform a garbage collection sweep on the repo",
		ShortDescription: `
'ipfs repo gc' is a plumbing command that will sweep the local
set of stored objects and remove ones that are not pinned in
order to reclaim hard disk space.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write minimal output"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		gcOutChan, err := corerepo.GarbageCollectBlockstore(n, req.Context().Context)
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

				var buf *bytes.Buffer
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
			}, nil
		},
	},
}
