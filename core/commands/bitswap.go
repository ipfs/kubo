package commands

import (
	"bytes"
	"fmt"
	"io"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	bitswap "gx/ipfs/QmQk1Rqy5XSBzXykMSsgiXfnhivCSnFpykx4M2j6DD1nBH/go-bitswap"
	decision "gx/ipfs/QmQk1Rqy5XSBzXykMSsgiXfnhivCSnFpykx4M2j6DD1nBH/go-bitswap/decision"

	"gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	peer "gx/ipfs/QmcZSzKEM5yDfpZbeEEZaVmaZ1zXm6JWTbrQZSB8hCVPzk/go-libp2p-peer"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Interact with the bitswap agent.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":      bitswapStatCmd,
		"wantlist":  lgc.NewCommand(showWantlistCmd),
		"unwant":    lgc.NewCommand(unwantCmd),
		"ledger":    lgc.NewCommand(ledgerCmd),
		"reprovide": lgc.NewCommand(reprovideCmd),
	},
}

var unwantCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove a given block from your wantlist.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "Key(s) to remove from your wantlist.").EnableStdin(),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			res.SetError(e.TypeErr(bs, nd.Exchange), cmdkit.ErrNormal)
			return
		}

		var ks []*cid.Cid
		for _, arg := range req.Arguments() {
			c, err := cid.Decode(arg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			ks = append(ks, c)
		}

		// TODO: This should maybe find *all* sessions for this request and cancel them?
		// (why): in reality, i think this command should be removed. Its
		// messing with the internal state of bitswap. You should cancel wants
		// by killing the command that caused the want.
		bs.CancelWants(ks, 0)

		res.SetOutput(nil)
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
			res.SetError(errNotOnline, cmdkit.ErrClient)
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
		nd, err := GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(errNotOnline, cmdkit.ErrClient)
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
			res.SetError(errNotOnline, cmdkit.ErrClient)
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
			res.SetError(errNotOnline, cmdkit.ErrClient)
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
