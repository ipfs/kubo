package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	p2p "github.com/ipfs/go-ipfs/p2p"

	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
)

// P2PListenerInfoOutput is output type of ls command
type P2PListenerInfoOutput struct {
	Protocol      string
	ListenAddress string
	TargetAddress string
}

// P2PStreamInfoOutput is output type of streams command
type P2PStreamInfoOutput struct {
	HandlerID     string
	Protocol      string
	LocalPeer     string
	LocalAddress  string
	RemotePeer    string
	RemoteAddress string
}

// P2PLsOutput is output type of ls command
type P2PLsOutput struct {
	Listeners []P2PListenerInfoOutput
}

// P2PStreamsOutput is output type of streams command
type P2PStreamsOutput struct {
	Streams []P2PStreamInfoOutput
}

// P2PCmd is the 'ipfs p2p' command
var P2PCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Libp2p stream mounting.",
		ShortDescription: `
Create and use tunnels to remote peers over libp2p

Note: this command is experimental and subject to change as usecases and APIs
are refined`,
	},

	Subcommands: map[string]*cmds.Command{
		"stream": p2pStreamCmd,

		"forward": p2pForwardCmd,
		"ls":      p2pLsCmd,
	},
}

var p2pForwardCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Forward connections to or from libp2p services",
		ShortDescription: `
Forward connections to <listen-address> to <target-address>. Protocol specifies
the libp2p protocol to use.

To create libp2p service listener, specify '/ipfs' as <listen-address>

Examples:
  ipfs p2p forward myproto /ipfs /ip4/127.0.0.1/tcp/1234
    - Forward connections to 'myproto' libp2p service to 127.0.0.1:1234

  ipfs p2p forward myproto /ip4/127.0.0.1/tcp/4567 /ipfs/QmPeer
    - Forward connections to 127.0.0.1:4567 to 'myproto' service on /ipfs/QmPeer

`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("protocol", true, false, "Protocol identifier."),
		cmdkit.StringArg("listen-address", true, false, "Listening endpoint"),
		cmdkit.StringArg("target-address", true, false, "Target endpoint."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		//TODO: Do we really want/need implicit prefix?
		proto := "/p2p/" + req.Arguments()[0]
		listen := req.Arguments()[1]
		target := req.Arguments()[2]

		if strings.HasPrefix(listen, "/ipfs") {
			if listen != "/ipfs" {
				res.SetError(errors.New("only '/ipfs' is allowed as libp2p listen address"), cmdkit.ErrNormal)
				return
			}

			if err := forwardRemote(n.Context(), n.P2P, proto, target); err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		} else {
			if err := forwardLocal(n.Context(), n.P2P, n.Peerstore, proto, listen, target); err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}
		res.SetOutput(nil)
	},
}

// forwardRemote forwards libp2p service connections to a manet address
func forwardRemote(ctx context.Context, p *p2p.P2P, proto string, target string) error {
	if strings.HasPrefix(target, "/ipfs") {
		return errors.New("cannot forward libp2p service connections to another libp2p service")
	}

	addr, err := ma.NewMultiaddr(target)
	if err != nil {
		return err
	}

	// TODO: return some info
	_, err = p.ForwardRemote(ctx, proto, addr)
	return err
}

// forwardLocal forwards local connections to a libp2p service
func forwardLocal(ctx context.Context, p *p2p.P2P, ps pstore.Peerstore, proto string, listen string, target string) error {
	bindAddr, err := ma.NewMultiaddr(listen)
	if err != nil {
		return err
	}

	addr, peer, err := ParsePeerParam(target)
	if err != nil {
		return err
	}

	if addr != nil {
		ps.AddAddr(peer, addr, pstore.TempAddrTTL)
	}

	// TODO: return some info
	_, err = p.ForwardLocal(ctx, peer, proto, bindAddr)
	return err
}

var p2pLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List active p2p listeners.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (Protocol, Listen, Target)."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output := &P2PLsOutput{}

		for _, listener := range n.P2P.Listeners.Listeners {
			output.Listeners = append(output.Listeners, P2PListenerInfoOutput{
				Protocol:      listener.Protocol(),
				ListenAddress: listener.ListenAddress(),
				TargetAddress: listener.TargetAddress(),
			})
		}

		res.SetOutput(output)
	},
	Type: P2PLsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			headers, _, _ := res.Request().Option("headers").Bool()
			list := v.(*P2PLsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, listener := range list.Listeners {
				if headers {
					fmt.Fprintln(w, "Protocol\tListen Address\tTarget Address")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", listener.Protocol, listener.ListenAddress, listener.TargetAddress)
			}
			w.Flush()

			return buf, nil
		},
	},
}

///////
// Listener
//

// p2pStreamCmd is the 'ipfs p2p stream' command
var p2pStreamCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "P2P stream management.",
		ShortDescription: "Create and manage p2p streams",
	},

	Subcommands: map[string]*cmds.Command{
		"ls":    p2pStreamLsCmd,
		"dial":  p2pStreamDialCmd,
		"close": p2pStreamCloseCmd,
	},
}

var p2pStreamLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List active p2p streams.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (HagndlerID, Protocol, Local, Remote)."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output := &P2PStreamsOutput{}

		for _, s := range n.P2P.Streams.Streams {
			output.Streams = append(output.Streams, P2PStreamInfoOutput{
				HandlerID: strconv.FormatUint(s.Id, 10),

				Protocol: s.Protocol,

				LocalPeer:    s.LocalPeer.Pretty(),
				LocalAddress: s.LocalAddr.String(),

				RemotePeer:    s.RemotePeer.Pretty(),
				RemoteAddress: s.RemoteAddr.String(),
			})
		}

		res.SetOutput(output)
	},
	Type: P2PStreamsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			headers, _, _ := res.Request().Option("headers").Bool()
			list := v.(*P2PStreamsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, stream := range list.Streams {
				if headers {
					fmt.Fprintln(w, "Id\tProtocol\tLocal\tRemote")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", stream.HandlerID, stream.Protocol, stream.LocalAddress, stream.RemotePeer)
			}
			w.Flush()

			return buf, nil
		},
	},
}

var p2pListenerListenCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Forward p2p connections to a network multiaddr.",
		ShortDescription: `
Register a p2p connection handler and forward the connections to a specified
address.

Note that the connections originate from the ipfs daemon process.
		`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("Protocol", true, false, "Protocol identifier."),
		cmdkit.StringArg("Address", true, false, "Request handling application address."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		proto := "/p2p/" + req.Arguments()[0]
		if n.P2P.CheckProtoExists(proto) {
			res.SetError(errors.New("protocol handler already registered"), cmdkit.ErrNormal)
			return
		}

		addr, err := ma.NewMultiaddr(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		_, err = n.P2P.ForwardRemote(n.Context(), proto, addr)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// Successful response.
		res.SetOutput(&P2PListenerInfoOutput{
			Protocol:      proto,
			TargetAddress: addr.String(),
		})
	},
}

var p2pStreamDialCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Dial to a p2p listener.",

		ShortDescription: `
Establish a new connection to a peer service.

When a connection is made to a peer service the ipfs daemon will setup one
time TCP listener and return it's bind port, this way a dialing application
can transparently connect to a p2p service.
		`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("Peer", true, false, "Remote peer to connect to"),
		cmdkit.StringArg("Protocol", true, false, "Protocol identifier."),
		cmdkit.StringArg("BindAddress", false, false, "Address to listen for connection/s (default: /ip4/127.0.0.1/tcp/0)."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		addr, peer, err := ParsePeerParam(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if addr != nil {
			n.Peerstore.AddAddr(peer, addr, pstore.TempAddrTTL)
		}

		proto := "/p2p/" + req.Arguments()[1]

		bindAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if len(req.Arguments()) == 3 {
			bindAddr, err = ma.NewMultiaddr(req.Arguments()[2])
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		listenerInfo, err := n.P2P.ForwardLocal(n.Context(), peer, proto, bindAddr)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output := P2PListenerInfoOutput{
			Protocol:      listenerInfo.Protocol(),
			ListenAddress: listenerInfo.ListenAddress(),
		}

		res.SetOutput(&output)
	},
}

var p2pListenerCloseCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Close active p2p listener.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("Protocol", false, false, "P2P listener protocol"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("all", "a", "Close all listeners."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(nil)

		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		closeAll, _, _ := req.Option("all").Bool()
		var proto string

		if !closeAll {
			if len(req.Arguments()) == 0 {
				res.SetError(errors.New("no protocol name specified"), cmdkit.ErrNormal)
				return
			}

			proto = "/p2p/" + req.Arguments()[0]
		}

		for _, listener := range n.P2P.Listeners.Listeners {
			if !closeAll && listener.Protocol() != proto {
				continue
			}
			listener.Close()
			if !closeAll {
				break
			}
		}
	},
}

var p2pStreamCloseCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Close active p2p stream.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("Id", false, false, "Stream Id"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("all", "a", "Close all streams."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(nil)

		n, err := getNode(req)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		closeAll, _, _ := req.Option("all").Bool()
		var handlerID uint64

		if !closeAll {
			if len(req.Arguments()) == 0 {
				res.SetError(errors.New("no Id specified"), cmdkit.ErrNormal)
				return
			}

			handlerID, err = strconv.ParseUint(req.Arguments()[0], 10, 64)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		for _, stream := range n.P2P.Streams.Streams {
			if !closeAll && handlerID != stream.Id {
				continue
			}
			stream.Reset()
			if !closeAll {
				break
			}
		}
	},
}

func getNode(req cmds.Request) (*core.IpfsNode, error) {
	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, err
	}

	config, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	if !config.Experimental.Libp2pStreamMounting {
		return nil, errors.New("libp2p stream mounting not enabled")
	}

	if !n.OnlineMode() {
		return nil, ErrNotOnline
	}

	return n, nil
}
