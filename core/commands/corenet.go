package commands

import (
	"errors"
	"io"
	"strconv"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"

	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	net "gx/ipfs/QmVHSBsn8LEeay8m5ERebgUVuhzw838PsyTttCmP6GMJkg/go-libp2p-net"
	peerstore "gx/ipfs/QmNUVzEjq3XWJ89hegahPvyfJbTXgTaom48pLb7YBD9gHQ/go-libp2p-peerstore"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

// Command output types.
type AppInfoOutput struct {
	Identity string
	Protocol string
	Address  string
}

type StreamInfoOutput struct {
	HandlerId     string
	Protocol      string
	LocalPeer     string
	LocalAddress  string
	RemotePeer    string
	RemoteAddress string
}

type ListCommandOutput struct {
	Apps    []AppInfoOutput
	Streams []StreamInfoOutput
}

// cnAppInfo holds information on a local application protocol listener service.
type cnAppInfo struct {
	// Application protocol identifier.
	protocol string

	// Node identity
	identity peer.ID

	// Local protocol stream address.
	address ma.Multiaddr

	// Local protocol stream listener.
	closer io.Closer

	// Flag indicating whether we're still accepting incoming connections, or
	// whether this application listener has been shutdown.
	running bool
}

func (c *cnAppInfo) Close() error {
	apps.Deregister(c.protocol)
	c.closer.Close()
	return nil
}

// cnAppRegistry is a collection of local application protocol listeners.
type cnAppRegistry struct {
	apps []*cnAppInfo
}

func (c *cnAppRegistry) Register(appInfo *cnAppInfo) {
	c.apps = append(c.apps, appInfo)
}

func (c *cnAppRegistry) Deregister(proto string) {
	foundAt := -1
	for i, a := range c.apps {
		if a.protocol == proto {
			foundAt = i
			break
		}
	}

	if foundAt != -1 {
		c.apps = append(c.apps[:foundAt], c.apps[foundAt+1:]...)
	}
}

// cnStreamInfo holds information on active incoming and outgoing protocol app streams.
type cnStreamInfo struct {
	handlerId uint64

	protocol string

	localPeer peer.ID
	localAddr ma.Multiaddr

	remotePeer peer.ID
	remoteAddr ma.Multiaddr

	local  io.ReadWriteCloser
	remote io.ReadWriteCloser
}

func (c *cnStreamInfo) Close() error {
	c.local.Close()
	c.remote.Close()
	streams.Deregister(c.handlerId)
	return nil
}

// cnStreamRegistry is a collection of active incoming and outgoing protocol app streams.
type cnStreamRegistry struct {
	streams []*cnStreamInfo

	nextId uint64
}

func (c *cnStreamRegistry) Register(streamInfo *cnStreamInfo) {
	streamInfo.handlerId = c.nextId
	c.streams = append(c.streams, streamInfo)
	c.nextId += 1
}

func (c *cnStreamRegistry) Deregister(handlerId uint64) {
	foundAt := -1
	for i, s := range c.streams {
		if s.handlerId == handlerId {
			foundAt = i
			break
		}
	}

	if foundAt != -1 {
		c.streams = append(c.streams[:foundAt], c.streams[foundAt+1:]...)
	}
}

//TODO: Ideally I'd like to see these combined into a module in core.
var apps cnAppRegistry
var streams cnStreamRegistry

var CorenetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Application network streams.",
	},

	Subcommands: map[string]*cmds.Command{
		"list":   listCmd,
		"dial":   dialCmd,
		"listen": listenCmd,
		"close":  closeCmd,
	},
}

var listCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List active application protocol connections.",
	},
	Options: []cmds.Option{
		cmds.BoolOption("apps", "a", "Display only local application protocol listeners.").Default(false),
		cmds.BoolOption("streams", "s", "Display active application protocol streams.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		var output ListCommandOutput

		for _, a := range apps.apps {
			output.Apps = append(output.Apps, AppInfoOutput{
				Identity: a.identity.Pretty(),
				Protocol: a.protocol,
				Address:  a.address.String(),
			})
		}

		for _, s := range streams.streams {
			output.Streams = append(output.Streams, StreamInfoOutput{
				HandlerId: strconv.FormatUint(s.handlerId, 10),

				Protocol: s.protocol,

				LocalPeer:    s.localPeer.Pretty(),
				LocalAddress: s.localAddr.String(),

				RemotePeer:    s.remotePeer.Pretty(),
				RemoteAddress: s.remoteAddr.String(),
			})
		}

		res.SetOutput(&output)
	},
}

var listenCmd = &cmds.Command{
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

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		proto := "/app/" + req.Arguments()[0]
		if checkProtoExists(n.PeerHost.Mux().Protocols(), proto) {
			res.SetError(errors.New("Protocol handler already registered."), cmds.ErrNormal)
			return
		}

		addr, err := ma.NewMultiaddr(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		listener, err := corenet.Listen(n, proto)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		app := cnAppInfo{
			identity: n.Identity,
			protocol: proto,
			address:  addr,
			closer:   listener,
			running:  true,
		}

		go acceptStreams(&app, listener)

		apps.Register(&app)

		// Successful response.
		res.SetOutput(&AppInfoOutput{
			Identity: app.identity.Pretty(),
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

func acceptStreams(app *cnAppInfo, listener corenet.Listener) {
	for app.running {
		remote, err := listener.Accept()
		if err != nil {
			listener.Close()
			break
		}

		local, err := manet.Dial(app.address)
		if err != nil {
			remote.Close()
			continue
		}

		stream := cnStreamInfo{
			protocol: app.protocol,

			localPeer: app.identity,
			localAddr: app.address,

			remotePeer: remote.Conn().RemotePeer(),
			remoteAddr: remote.Conn().RemoteMultiaddr(),

			local:  local,
			remote: remote,
		}

		streams.Register(&stream)
		startStreaming(&stream)
	}
	apps.Deregister(app.protocol)
}

func startStreaming(stream *cnStreamInfo) {
	go func() {
		io.Copy(stream.local, stream.remote)
		stream.Close()
	}()

	go func() {
		io.Copy(stream.remote, stream.local)
		stream.Close()
	}()
}

var dialCmd = &cmds.Command{
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

		app := cnAppInfo{
			identity: n.Identity,
			protocol: proto,
		}

		n.Peerstore.AddAddr(peer, addr, peerstore.TempAddrTTL)

		remote, err := corenet.Dial(n, peer, proto)
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

			app.address = listener.Multiaddr()
			app.closer = listener
			app.running = true

			go doAccept(&app, remote, listener)

		default:
			res.SetError(errors.New("Unsupported protocol: "+lnet), cmds.ErrNormal)
			return
		}

		output := AppInfoOutput{
			Identity: app.identity.Pretty(),
			Protocol: app.protocol,
			Address:  app.address.String(),
		}

		res.SetOutput(&output)
	},
}

func doAccept(app *cnAppInfo, remote net.Stream, listener manet.Listener) {
	defer listener.Close()

	local, err := listener.Accept()
	if err != nil {
		return
	}

	stream := cnStreamInfo{
		protocol: app.protocol,

		localPeer: app.identity,
		localAddr: app.address,

		remotePeer: remote.Conn().RemotePeer(),
		remoteAddr: remote.Conn().RemoteMultiaddr(),

		local:  local,
		remote: remote,
	}

	streams.Register(&stream)
	startStreaming(&stream)
}

var closeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Closes an active stream listener or client.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("HandlerId", false, false, "Application listener or client HandlerId"),
		cmds.StringArg("Protocol", false, false, "Application listener or client HandlerId"),
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

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		closeAll, _, _ := req.Option("all").Bool()

		var proto string
		var handlerId uint64

		useHandlerId := false

		if !closeAll && len(req.Arguments()) == 0 {
			res.SetError(errors.New("You must supply a handlerId or stream protocol."), cmds.ErrNormal)
			return

		} else if !closeAll {
			handlerId, err = strconv.ParseUint(req.Arguments()[0], 10, 64)
			if err != nil {
				proto = "/app/" + req.Arguments()[0]

			} else {
				useHandlerId = true
			}
		}

		if closeAll || useHandlerId {
			for _, s := range streams.streams {
				if !closeAll && handlerId != s.handlerId {
					continue
				}
				s.Close()
				if !closeAll {
					break
				}
			}
		}

		if closeAll || !useHandlerId {
			for _, a := range apps.apps {
				if !closeAll && a.protocol != proto {
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
