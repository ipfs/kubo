package commands

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"
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
	Options: []cmds.Option{
		cmds.StringOption("peer", "p", "specify which peer to show wantlist for (default self)"),
	},
	Type: KeyList{},
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

		pstr, found, err := req.Option("peer").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if found {
			pid, err := peer.IDB58Decode(pstr)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&KeyList{bs.WantlistForPeer(pid)})
		} else {
			res.SetOutput(&KeyList{bs.GetWantlist()})
		}
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
			fmt.Fprintln(buf, "bitswap status")
			fmt.Fprintf(buf, "\tprovides buffer: %d / %d\n", out.ProvideBufLen, bitswap.HasBlockBufferSize)
			fmt.Fprintf(buf, "\twantlist [%d keys]\n", len(out.Wantlist))
			for _, k := range out.Wantlist {
				fmt.Fprintf(buf, "\t\t%s\n", k.B58String())
			}
			fmt.Fprintf(buf, "\tpartners [%d]\n", len(out.Peers))
			for _, p := range out.Peers {
				fmt.Fprintf(buf, "\t\t%s\n", p)
			}
			return buf, nil
		},
	},
}
