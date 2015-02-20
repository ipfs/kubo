package commands

import (
	"bytes"
	"encoding/json"
	cmds "github.com/jbenet/go-ipfs/commands"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	u "github.com/jbenet/go-ipfs/util"
	"io"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "A set of commands to manipulate the bitswap agent",
		ShortDescription: ``,
	},
	Subcommands: map[string]*cmds.Command{
		"wantlist": showWantlistCmd,
		"stat":     bitswapStatCmd,
	},
}

var showWantlistCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show blocks currently on the wantlist",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer`,
	},
	Type: KeyList{},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyList{nd.Exchange.GetWantlist()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: KeyListTextMarshaler,
	},
}

var bitswapStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "show some diagnostic information on the bitswap agent",
		ShortDescription: ``,
	},
	Type: bitswap.Stat{},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}

		st, err := bs.Stat()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(st)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out, ok := res.Output().(*bitswap.Stat)
			if !ok {
				return nil, u.ErrCast()
			}
			buf := new(bytes.Buffer)
			enc := json.NewEncoder(buf)
			err := enc.Encode(out)
			if err != nil {
				return nil, err
			}
			return buf, nil
		},
	},
}
