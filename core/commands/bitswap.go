package commands

import (
	"bytes"
	"fmt"
	"io"

	bitswap "github.com/ipfs/go-bitswap"
	decision "github.com/ipfs/go-bitswap/decision"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	"github.com/dustin/go-humanize"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-peer"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Interact with the bitswap agent.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":      bitswapStatCmd,
		"wantlist":  lgc.NewCommand(showWantlistCmd),
		"ledger":    lgc.NewCommand(ledgerCmd),
		"reprovide": lgc.NewCommand(reprovideCmd),
	},
}

var showWantlistCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show blocks currently on the wantlist.",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer.`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("peer", "p", "Specify which peer to show wantlist for. Default: self."),
	},
	Type: KeyList{},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(ErrNotOnline, cmdkit.ErrClient)
			return
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			res.SetError(e.TypeErr(bs, nd.Exchange), cmdkit.ErrNormal)
			return
		}

		pstr, found, err := req.Option("peer").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if found {
			pid, err := peer.IDB58Decode(pstr)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			if pid == nd.Identity {
				res.SetOutput(&KeyList{bs.GetWantlist()})
				return
			}

			res.SetOutput(&KeyList{bs.WantlistForPeer(pid)})
		} else {
			res.SetOutput(&KeyList{bs.GetWantlist()})
		}
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: KeyListTextMarshaler,
	},
}

var bitswapStatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show some diagnostic information on the bitswap agent.",
		ShortDescription: ``,
	},
	Type: bitswap.Stat{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(ErrNotOnline, cmdkit.ErrClient)
			return
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			res.SetError(e.TypeErr(bs, nd.Exchange), cmdkit.ErrNormal)
			return
		}

		st, err := bs.Stat()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		cmds.EmitOnce(res, st)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			out, ok := v.(*bitswap.Stat)
			if !ok {
				return e.TypeErr(out, v)
			}

			fmt.Fprintln(w, "bitswap status")
			fmt.Fprintf(w, "\tprovides buffer: %d / %d\n", out.ProvideBufLen, bitswap.HasBlockBufferSize)
			fmt.Fprintf(w, "\tblocks received: %d\n", out.BlocksReceived)
			fmt.Fprintf(w, "\tblocks sent: %d\n", out.BlocksSent)
			fmt.Fprintf(w, "\tdata received: %d\n", out.DataReceived)
			fmt.Fprintf(w, "\tdata sent: %d\n", out.DataSent)
			fmt.Fprintf(w, "\tdup blocks received: %d\n", out.DupBlksReceived)
			fmt.Fprintf(w, "\tdup data received: %s\n", humanize.Bytes(out.DupDataReceived))
			fmt.Fprintf(w, "\twantlist [%d keys]\n", len(out.Wantlist))
			for _, k := range out.Wantlist {
				fmt.Fprintf(w, "\t\t%s\n", k.String())
			}
			fmt.Fprintf(w, "\tpartners [%d]\n", len(out.Peers))
			for _, p := range out.Peers {
				fmt.Fprintf(w, "\t\t%s\n", p)
			}

			return nil
		}),
	},
}

var ledgerCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show the current ledger for a peer.",
		ShortDescription: `
The Bitswap decision engine tracks the number of bytes exchanged between IPFS
nodes, and stores this information as a collection of ledgers. This command
prints the ledger associated with a given peer.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer", true, false, "The PeerID (B58) of the ledger to inspect."),
	},
	Type: decision.Receipt{},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(ErrNotOnline, cmdkit.ErrClient)
			return
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			res.SetError(e.TypeErr(bs, nd.Exchange), cmdkit.ErrNormal)
			return
		}

		partner, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrClient)
			return
		}
		res.SetOutput(bs.LedgerForPeer(partner))
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*decision.Receipt)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "Ledger for %s\n"+
				"Debt ratio:\t%f\n"+
				"Exchanges:\t%d\n"+
				"Bytes sent:\t%d\n"+
				"Bytes received:\t%d\n\n",
				out.Peer, out.Value, out.Exchanged,
				out.Sent, out.Recv)
			return buf, nil
		},
	},
}

var reprovideCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce our data to network.
`,
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(ErrNotOnline, cmdkit.ErrClient)
			return
		}

		err = nd.Reprovider.Trigger(req.Context())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(nil)
	},
}
