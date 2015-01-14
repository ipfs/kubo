package commands

import (
	"bytes"
	"fmt"
	"io"
	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
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

type KeyRemoved struct {
	Key u.Key
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
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		keychan, err := n.Blockstore.AllKeysChan(req.Context().Context, 0, 1<<16)
		if err != nil {
			return nil, err
		}

		outChan := make(chan interface{})
		go GarbageCollectBlockstore(n, keychan, outChan)
		return outChan, nil
	},
	Type: KeyRemoved{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			quiet, _, err := res.Request().Option("quiet").Bool()
			if err != nil {
				return nil, err
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*KeyRemoved)
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

func GarbageCollectBlockstore(n *core.IpfsNode, keychan <-chan u.Key, output chan interface{}) {
	defer close(output)
	for k := range keychan {
		if !n.Pinning.IsPinned(k) {
			err := n.Blockstore.DeleteBlock(k)
			if err != nil {
				log.Errorf("Error removing key from blockstore: %s", err)
			}
			output <- &KeyRemoved{k}
		}
	}
}
