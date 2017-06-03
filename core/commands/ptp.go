package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
)

// PTPListenerInfoOutput is output type of ls command
type PTPListenerInfoOutput struct {
	Protocol string
	Address  string
}

// PTPStreamInfoOutput is output type of streams command
type PTPStreamInfoOutput struct {
	HandlerID     string
	Protocol      string
	LocalPeer     string
	LocalAddress  string
	RemotePeer    string
	RemoteAddress string
}

// PTPLsOutput is output type of ls command
type PTPLsOutput struct {
	Listeners []PTPListenerInfoOutput
}

// PTPStreamsOutput is output type of streams command
type PTPStreamsOutput struct {
	Streams []PTPStreamInfoOutput
}

// PTPCmd is the 'ipfs ptp' command
var PTPCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Libp2p stream mounting.",
		ShortDescription: `
Create and use tunnels to remote peers over libp2p

Note: this command is experimental and subject to change as usecases and APIs are refined`,
	},

	Subcommands: map[string]*cmds.Command{
		"ls":      ptpLsCmd,
		"streams": ptpStreamsCmd,
		"dial":    ptpDialCmd,
		"listen":  ptpListenCmd,
		"close":   ptpCloseCmd,
	},
}

var ptpLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List active p2p listeners.",
	},
	Options: []cmds.Option{
		cmds.BoolOption("headers", "v", "Print table headers (HandlerID, Protocol, Local, Remote).").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = checkEnabled(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		output := &PTPLsOutput{}

		for _, listener := range n.PTP.Listeners.Listeners {
			output.Listeners = append(output.Listeners, PTPListenerInfoOutput{
				Protocol: listener.Protocol,
				Address:  listener.Address.String(),
			})
		}

		res.SetOutput(output)
	},
	Type: PTPLsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			headers, _, _ := res.Request().Option("headers").Bool()
			list, _ := res.Output().(*PTPLsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, listener := range list.Listeners {
				if headers {
					fmt.Fprintln(w, "Address\tProtocol")
				}

				fmt.Fprintf(w, "%s\t%s\n", listener.Address, listener.Protocol)
			}
			w.Flush()

			return buf, nil
		},
	},
}

var ptpStreamsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List active p2p streams.",
	},
	Options: []cmds.Option{
		cmds.BoolOption("headers", "v", "Print table headers (HagndlerID, Protocol, Local, Remote).").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = checkEnabled(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		output := &PTPStreamsOutput{}

		for _, s := range n.PTP.Streams.Streams {
			output.Streams = append(output.Streams, PTPStreamInfoOutput{
				HandlerID: strconv.FormatUint(s.HandlerID, 10),

				Protocol: s.Protocol,

				LocalPeer:    s.LocalPeer.Pretty(),
				LocalAddress: s.LocalAddr.String(),

				RemotePeer:    s.RemotePeer.Pretty(),
				RemoteAddress: s.RemoteAddr.String(),
			})
		}

		res.SetOutput(output)
	},
	Type: PTPStreamsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			headers, _, _ := res.Request().Option("headers").Bool()
			list, _ := res.Output().(*PTPStreamsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, stream := range list.Streams {
				if headers {
					fmt.Fprintln(w, "HandlerID\tProtocol\tLocal\tRemote")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", stream.HandlerID, stream.Protocol, stream.LocalAddress, stream.RemotePeer)
			}
			w.Flush()

			return buf, nil
		},
	},
}

var ptpListenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Forward p2p connections to a network multiaddr.",
		ShortDescription: `
Register a p2p connection handler and forward the connections to a specified address.

Note that the connections originate from the ipfs daemon process.
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Protocol", true, false, "Protocol identifier."),
		cmds.StringArg("Address", true, false, "Request handling application address."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = checkEnabled(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		proto := "/ptp/" + req.Arguments()[0]
		if n.PTP.CheckProtoExists(proto) {
			res.SetError(errors.New("protocol handler already registered"), cmds.ErrNormal)
			return
		}

		addr, err := ma.NewMultiaddr(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		_, err = n.PTP.NewListener(n.Context(), proto, addr)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Successful response.
		res.SetOutput(&PTPListenerInfoOutput{
			Protocol: proto,
			Address:  addr.String(),
		})
	},
}

var ptpDialCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Dial to a p2p listener.",

		ShortDescription: `
Establish a new connection to a peer service.

When a connection is made to a peer service the ipfs daemon will setup one time
TCP listener and return it's bind port, this way a dialing application can
transparently connect to a p2p service.
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Peer", true, false, "Remote peer to connect to"),
		cmds.StringArg("Protocol", true, false, "Protocol identifier."),
		cmds.StringArg("BindAddress", false, false, "Address to listen for connection/s (default: /ip4/127.0.0.1/tcp/0)."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = checkEnabled(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		addr, peer, err := ParsePeerParam(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		proto := "/ptp/" + req.Arguments()[1]

		bindAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if len(req.Arguments()) == 3 {
			bindAddr, err = ma.NewMultiaddr(req.Arguments()[2])
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		listenerInfo, err := n.PTP.Dial(n.Context(), addr, peer, proto, bindAddr)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output := PTPListenerInfoOutput{
			Protocol: listenerInfo.Protocol,
			Address:  listenerInfo.Address.String(),
		}

		res.SetOutput(&output)
	},
}

var ptpCloseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Closes an active p2p stream or listener.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Identifier", false, false, "Stream HandlerID or p2p listener protocol"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("all", "a", "Close all streams and listeners.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = checkEnabled(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		closeAll, _, _ := req.Option("all").Bool()

		var proto string
		var handlerID uint64

		useHandlerID := false

		if !closeAll {
			if len(req.Arguments()) == 0 {
				res.SetError(errors.New("no handlerID nor listener protocol specified"), cmds.ErrNormal)
				return
			}

			handlerID, err = strconv.ParseUint(req.Arguments()[0], 10, 64)
			if err != nil {
				proto = "/ptp/" + req.Arguments()[0]
			} else {
				useHandlerID = true
			}
		}

		if closeAll || useHandlerID {
			for _, stream := range n.PTP.Streams.Streams {
				if !closeAll && handlerID != stream.HandlerID {
					continue
				}
				stream.Close()
				if !closeAll {
					break
				}
			}
		}

		if closeAll || !useHandlerID {
			for _, listener := range n.PTP.Listeners.Listeners {
				if !closeAll && listener.Protocol != proto {
					continue
				}
				listener.Close()
				if !closeAll {
					break
				}
			}
		}
	},
}

func checkEnabled(n *core.IpfsNode) error {
	config, err := n.Repo.Config()
	if err != nil {
		return err
	}

	if !config.Experimental.Libp2pStreamMounting {
		return errors.New("libp2p stream mounting not enabled")
	}
	return nil
}
