package commands

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"time"

	cid "github.com/ipfs/go-cid"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	mh "github.com/multiformats/go-multihash"

	humanize "github.com/dustin/go-humanize"
	bitswap "github.com/ipfs/go-bitswap"
	decision "github.com/ipfs/go-bitswap/decision"
	bsmsg "github.com/ipfs/go-bitswap/message"
	bsmsgpb "github.com/ipfs/go-bitswap/message/pb"
	bsnet "github.com/ipfs/go-bitswap/network"
	cidutil "github.com/ipfs/go-cidutil"
	cmds "github.com/ipfs/go-ipfs-cmds"
	nrouting "github.com/ipfs/go-ipfs-routing/none"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Interact with the bitswap agent.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":      bitswapStatCmd,
		"wantlist":  showWantlistCmd,
		"ledger":    ledgerCmd,
		"reprovide": reprovideCmd,
		"sanity":    bitswapSanityCheckCmd,
	},
}

const (
	peerOptionName = "peer"
)

var showWantlistCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show blocks currently on the wantlist.",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer.`,
	},
	Options: []cmds.Option{
		cmds.StringOption(peerOptionName, "p", "Specify which peer to show wantlist for. Default: self."),
	},
	Type: KeyList{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		pstr, found := req.Options[peerOptionName].(string)
		if found {
			pid, err := peer.Decode(pstr)
			if err != nil {
				return err
			}
			if pid != nd.Identity {
				return cmds.EmitOnce(res, &KeyList{bs.WantlistForPeer(pid)})
			}
		}

		return cmds.EmitOnce(res, &KeyList{bs.GetWantlist()})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *KeyList) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			// sort the keys first
			cidutil.Sort(out.Keys)
			for _, key := range out.Keys {
				fmt.Fprintln(w, enc.Encode(key))
			}
			return nil
		}),
	},
}

const (
	bitswapVerboseOptionName = "verbose"
	bitswapHumanOptionName   = "human"
)

var bitswapStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show some diagnostic information on the bitswap agent.",
		ShortDescription: ``,
	},
	Options: []cmds.Option{
		cmds.BoolOption(bitswapVerboseOptionName, "v", "Print extra information"),
		cmds.BoolOption(bitswapHumanOptionName, "Print sizes in human readable format (e.g., 1K 234M 2G)"),
	},
	Type: bitswap.Stat{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return cmds.Errorf(cmds.ErrClient, ErrNotOnline.Error())
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		st, err := bs.Stat()
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, st)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s *bitswap.Stat) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			verbose, _ := req.Options[bitswapVerboseOptionName].(bool)
			human, _ := req.Options[bitswapHumanOptionName].(bool)

			fmt.Fprintln(w, "bitswap status")
			fmt.Fprintf(w, "\tprovides buffer: %d / %d\n", s.ProvideBufLen, bitswap.HasBlockBufferSize)
			fmt.Fprintf(w, "\tblocks received: %d\n", s.BlocksReceived)
			fmt.Fprintf(w, "\tblocks sent: %d\n", s.BlocksSent)
			if human {
				fmt.Fprintf(w, "\tdata received: %s\n", humanize.Bytes(s.DataReceived))
				fmt.Fprintf(w, "\tdata sent: %s\n", humanize.Bytes(s.DataSent))
			} else {
				fmt.Fprintf(w, "\tdata received: %d\n", s.DataReceived)
				fmt.Fprintf(w, "\tdata sent: %d\n", s.DataSent)
			}
			fmt.Fprintf(w, "\tdup blocks received: %d\n", s.DupBlksReceived)
			if human {
				fmt.Fprintf(w, "\tdup data received: %s\n", humanize.Bytes(s.DupDataReceived))
			} else {
				fmt.Fprintf(w, "\tdup data received: %d\n", s.DupDataReceived)
			}
			fmt.Fprintf(w, "\twantlist [%d keys]\n", len(s.Wantlist))
			for _, k := range s.Wantlist {
				fmt.Fprintf(w, "\t\t%s\n", enc.Encode(k))
			}

			fmt.Fprintf(w, "\tpartners [%d]\n", len(s.Peers))
			if verbose {
				for _, p := range s.Peers {
					fmt.Fprintf(w, "\t\t%s\n", p)
				}
			}

			return nil
		}),
	},
}

var ledgerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the current ledger for a peer.",
		ShortDescription: `
The Bitswap decision engine tracks the number of bytes exchanged between IPFS
nodes, and stores this information as a collection of ledgers. This command
prints the ledger associated with a given peer.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer", true, false, "The PeerID (B58) of the ledger to inspect."),
	},
	Type: decision.Receipt{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		partner, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, bs.LedgerForPeer(partner))
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *decision.Receipt) error {
			fmt.Fprintf(w, "Ledger for %s\n"+
				"Debt ratio:\t%f\n"+
				"Exchanges:\t%d\n"+
				"Bytes sent:\t%d\n"+
				"Bytes received:\t%d\n\n",
				out.Peer, out.Value, out.Exchanged,
				out.Sent, out.Recv)
			return nil
		}),
	},
}

var reprovideCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce our data to network.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		err = nd.Provider.Reprovide(req.Context)
		if err != nil {
			return err
		}

		return nil
	},
}

var bitswapSanityCheckCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("timeout", false, false, "test timeout"),
		cmds.StringArg("num", false, false, "CID count"),
		cmds.StringArg("target", true, false, "target AddrInfo string"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		timeout, err := time.ParseDuration(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("timeout must be time duration. %w", err)
		}
		cidNum, err := strconv.Atoi(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("cidNum must be an integer")
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		target, err := peer.AddrInfoFromString(req.Arguments[2])
		if err != nil {
			return err
		}
		err = nd.PeerHost.Connect(ctx, *target)
		if err != nil {
			return err
		}

		nilRouter, err := nrouting.ConstructNilRouting(nil, nil, nil, nil)
		if err != nil {
			return err
		}
		c := make(chan interface{})
		rcv := &bsReceiver{
			target: target.ID,
			result: c,
		}
		bn := bsnet.NewFromIpfsHost(nd.PeerHost, nilRouter)
		bn.SetDelegate(rcv)

		testCids := createCids(cidNum)
		msg := createMsg(testCids)
		if err := bn.SendMessage(ctx, target.ID, msg); err != nil {
			return nil
		}
		cmds.EmitChan(res, c)
		return nil
	},
	Type: msgOrErr{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *msgOrErr) error {
			_, err := fmt.Fprintf(w, "%v\n", out)
			return err
		}),
	},
}

type bsReceiver struct {
	target peer.ID
	result chan interface{}
}

type msgOrErr struct {
	msg bsmsg.BitSwapMessage
	err error
}

func (r *bsReceiver) ReceiveMessage(ctx context.Context, sender peer.ID, incoming bsmsg.BitSwapMessage) {
	fmt.Println(incoming)
	if r.target != sender {
		select {
		case <-ctx.Done():
		case r.result <- &msgOrErr{err: fmt.Errorf("expected peerID %v, got %v", r.target, sender)}:
		}
		return
	}

	select {
	case <-ctx.Done():
	case r.result <- &msgOrErr{msg: incoming}:
	}
}

func (r *bsReceiver) ReceiveError(err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
	case r.result <- &msgOrErr{err: err}:
	}
}

func (r *bsReceiver) PeerConnected(id peer.ID) {
	return
}

func (r *bsReceiver) PeerDisconnected(id peer.ID) {
	return
}

func createCids(cidNum int) map[cid.Cid]bool {
	cids := make(map[cid.Cid]bool, cidNum)

	v1RawIDPrefix := cid.Prefix{
		Version: 1,
		Codec:   cid.Raw,
		MhType:  mh.IDENTITY,
	}

	v1RawSha256Prefix := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.SHA2_256,
		MhLength: 20, // To guarantee nonexistence shorten the 32-byte SHA length.
	}

	cidsPerGroup := int(math.Ceil(float64(cidNum) / 2))
	for i := 0; i < cidsPerGroup; i++ {
		c, err := v1RawIDPrefix.Sum([]byte(strconv.FormatUint(uint64(i), 10)))
		if err != nil {
			panic("error creating raw ID CID")
		}
		cids[c] = false
		c, err = v1RawSha256Prefix.Sum([]byte(strconv.FormatUint(uint64(i), 10)))
		if err != nil {
			panic("error creating raw SHA-256 CID")
		}
		cids[c] = false
	}
	return cids
}

func createMsg(requestedCids map[cid.Cid]bool) bsmsg.BitSwapMessage {
	msg := bsmsg.New(true)
	// Creating a new full list to replace the previous one since each test
	// is independent.
	for c := range requestedCids {
		wantType := bsmsgpb.Message_Wantlist_Have
		if rand.Intn(2) == 1 {
			wantType = bsmsgpb.Message_Wantlist_Block
		}
		msg.AddEntry(c, rand.Int31(), wantType, true)
		// Priority is only local to peer so not relevant for this test, still
		// keep it random just in case to avoid bias.
	}
	return msg
}
