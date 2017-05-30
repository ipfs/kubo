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
	corenet "github.com/ipfs/go-ipfs/corenet"
	cnet "github.com/ipfs/go-ipfs/corenet/net"

	peerstore "gx/ipfs/QmNUVzEjq3XWJ89hegahPvyfJbTXgTaom48pLb7YBD9gHQ/go-libp2p-peerstore"
	net "gx/ipfs/QmVHSBsn8LEeay8m5ERebgUVuhzw838PsyTttCmP6GMJkg/go-libp2p-net"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

// CorenetAppInfoOutput is output type of ls command
type CorenetAppInfoOutput struct {
	Protocol string
	Address  string
}

// CorenetStreamInfoOutput is output type of streams command
type CorenetStreamInfoOutput struct {
	HandlerID     string
	Protocol      string
	LocalPeer     string
	LocalAddress  string
	RemotePeer    string
	RemoteAddress string
}

// CorenetLsOutput is output type of ls command
type CorenetLsOutput struct {
	Apps []CorenetAppInfoOutput
}

// CorenetStreamsOutput is output type of streams command
type CorenetStreamsOutput struct {
	Streams []CorenetStreamInfoOutput
}

var CorenetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Libp2p stream mounting.",
		ShortDescription: `
Expose a local application to remote peers over libp2p

Note: this command is experimental and subject to change as usecases and APIs are refined`,
	},

	Subcommands: map[string]*cmds.Command{
		"ls":      CorenetLsCmd,
		"streams": CorenetStreamsCmd,
		"dial":    CorenetDialCmd,
		"listen":  CorenetListenCmd,
		"close":   CorenetCloseCmd,
	},
}

var CorenetLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List active application protocol listeners.",
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

		output := &CorenetLsOutput{}

		for _, app := range n.Corenet.Apps.Apps {
			output.Apps = append(output.Apps, CorenetAppInfoOutput{
				Protocol: app.Protocol,
				Address:  app.Address.String(),
			})
		}

		res.SetOutput(output)
	},
	Type: CorenetLsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			headers, _, _ := res.Request().Option("headers").Bool()
			list, _ := res.Output().(*CorenetLsOutput)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, app := range list.Apps {
				if headers {
					fmt.Fprintln(w, "Address\tProtocol")
				}

				fmt.Fprintf(w, "%s\t%s\n", app.Address, app.Protocol)
			}
			w.Flush()

			return buf, nil
		},
	},
}

var CorenetStreamsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List active application protocol streams.",
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

		output := &CorenetStreamsOutput{}

		for _, s := range n.Corenet.Streams.Streams {
			output.Streams = append(output.Streams, CorenetStreamInfoOutput{
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
	Type: CorenetStreamsOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			headers, _, _ := res.Request().Option("headers").Bool()
			list, _ := res.Output().(*CorenetStreamsOutput)
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

var CorenetListenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create application protocol listener and proxy to network multiaddr.",
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

		proto := "/app/" + req.Arguments()[0]
		if checkProtoExists(n.PeerHost.Mux().Protocols(), proto) {
			res.SetError(errors.New("protocol handler already registered"), cmds.ErrNormal)
			return
		}

		addr, err := ma.NewMultiaddr(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		listener, err := cnet.Listen(n, proto)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		app := corenet.AppInfo{
			Identity: n.Identity,
			Protocol: proto,
			Address:  addr,
			Closer:   listener,
			Running:  true,
			Registry: &n.Corenet.Apps,
		}

		go acceptStreams(n, &app, listener)

		n.Corenet.Apps.Register(&app)

		// Successful response.
		res.SetOutput(&CorenetAppInfoOutput{
			Protocol: proto,
			Address:  addr.String(),
		})
	},
}

func checkProtoExists(protos []string, proto string) bool {
	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}

func acceptStreams(n *core.IpfsNode, app *corenet.AppInfo, listener cnet.Listener) {
	for app.Running {
		remote, err := listener.Accept()
		if err != nil {
			listener.Close()
			break
		}

		local, err := manet.Dial(app.Address)
		if err != nil {
			remote.Close()
			continue
		}

		stream := corenet.StreamInfo{
			Protocol: app.Protocol,

			LocalPeer: app.Identity,
			LocalAddr: app.Address,

			RemotePeer: remote.Conn().RemotePeer(),
			RemoteAddr: remote.Conn().RemoteMultiaddr(),

			Local:  local,
			Remote: remote,

			Registry: &n.Corenet.Streams,
		}

		n.Corenet.Streams.Register(&stream)
		startStreaming(&stream)
	}
	n.Corenet.Apps.Deregister(app.Protocol)
}

func startStreaming(stream *corenet.StreamInfo) {
	go func() {
		io.Copy(stream.Local, stream.Remote)
		stream.Close()
	}()

	go func() {
		io.Copy(stream.Remote, stream.Local)
		stream.Close()
	}()
}

var CorenetDialCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Dial to an application service.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Peer", true, false, "Remote peer to connect to"),
		cmds.StringArg("Protocol", true, false, "Protocol identifier."),
		cmds.StringArg("BindAddress", false, false, "Address to listen for application/s (default: /ip4/127.0.0.1/tcp/0)."),
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

		proto := "/app/" + req.Arguments()[1]

		bindAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if len(req.Arguments()) == 3 {
			bindAddr, err = ma.NewMultiaddr(req.Arguments()[2])
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		lnet, _, err := manet.DialArgs(bindAddr)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		app := corenet.AppInfo{
			Identity: n.Identity,
			Protocol: proto,
		}

		n.Peerstore.AddAddr(peer, addr, peerstore.TempAddrTTL)

		remote, err := cnet.Dial(n, peer, proto)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		switch lnet {
		case "tcp", "tcp4", "tcp6":
			listener, err := manet.Listen(bindAddr)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				if err := remote.Close(); err != nil {
					res.SetError(err, cmds.ErrNormal)
				}
				return
			}

			app.Address = listener.Multiaddr()
			app.Closer = listener
			app.Running = true

			go doAccept(n, &app, remote, listener)

		default:
			res.SetError(errors.New("unsupported protocol: "+lnet), cmds.ErrNormal)
			return
		}

		output := CorenetAppInfoOutput{
			Protocol: app.Protocol,
			Address:  app.Address.String(),
		}

		res.SetOutput(&output)
	},
}

func doAccept(n *core.IpfsNode, app *corenet.AppInfo, remote net.Stream, listener manet.Listener) {
	defer listener.Close()

	local, err := listener.Accept()
	if err != nil {
		return
	}

	stream := corenet.StreamInfo{
		Protocol: app.Protocol,

		LocalPeer: app.Identity,
		LocalAddr: app.Address,

		RemotePeer: remote.Conn().RemotePeer(),
		RemoteAddr: remote.Conn().RemoteMultiaddr(),

		Local:  local,
		Remote: remote,

		Registry: &n.Corenet.Streams,
	}

	n.Corenet.Streams.Register(&stream)
	startStreaming(&stream)
}

var CorenetCloseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Closes an active stream listener or client.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("HandlerID", false, false, "Application listener or client HandlerID"),
		cmds.StringArg("Protocol", false, false, "Application listener or client HandlerID"),
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

		if !closeAll && len(req.Arguments()) == 0 {
			res.SetError(errors.New(" handlerID nor stream protocol"), cmds.ErrNormal)
			return

		} else if !closeAll {
			handlerID, err = strconv.ParseUint(req.Arguments()[0], 10, 64)
			if err != nil {
				proto = "/app/" + req.Arguments()[0]

			} else {
				useHandlerID = true
			}
		}

		if closeAll || useHandlerID {
			for _, s := range n.Corenet.Streams.Streams {
				if !closeAll && handlerID != s.HandlerID {
					continue
				}
				s.Close()
				if !closeAll {
					break
				}
			}
		}

		if closeAll || !useHandlerID {
			for _, a := range n.Corenet.Apps.Apps {
				if !closeAll && a.Protocol != proto {
					continue
				}
				a.Close()
				if !closeAll {
					break
				}
			}
		}

		if len(req.Arguments()) != 1 {
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
