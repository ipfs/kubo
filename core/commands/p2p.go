package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	p2p "github.com/ipfs/go-ipfs/p2p"

	ipfsaddr "github.com/ipfs/go-ipfs-addr"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
)

// P2PProtoPrefix is the default required prefix for protocol names
const P2PProtoPrefix = "/x/"

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
	OriginAddress string
	TargetAddress string
}

// P2PLsOutput is output type of ls command
type P2PLsOutput struct {
	Listeners []P2PListenerInfoOutput
}

// P2PStreamsOutput is output type of streams command
type P2PStreamsOutput struct {
	Streams []P2PStreamInfoOutput
}

const (
	allowCustomProtocolOptionName = "allow-custom-protocol"
	reportPeerIDOptionName        = "report-peer-id"
)

var resolveTimeout = 10 * time.Second

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
		"stream":  p2pStreamCmd,
		"forward": p2pForwardCmd,
		"listen":  p2pListenCmd,
		"close":   p2pCloseCmd,
		"ls":      p2pLsCmd,
	},
}

var p2pForwardCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Forward connections to libp2p service",
		ShortDescription: `
Forward connections made to <listen-address> to <target-address>.

<protocol> specifies the libp2p protocol name to use for libp2p
connections and/or handlers. It must be prefixed with '` + P2PProtoPrefix + `'.

Example:
  ipfs p2p forward ` + P2PProtoPrefix + `myproto /ip4/127.0.0.1/tcp/4567 /ipfs/QmPeer
    - Forward connections to 127.0.0.1:4567 to '` + P2PProtoPrefix + `myproto' service on /ipfs/QmPeer

`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("protocol", true, false, "Protocol name."),
		cmdkit.StringArg("listen-address", true, false, "Listening endpoint."),
		cmdkit.StringArg("target-address", true, false, "Target endpoint."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(allowCustomProtocolOptionName, "Don't require /x/ prefix"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		protoOpt := req.Arguments[0]
		listenOpt := req.Arguments[1]
		targetOpt := req.Arguments[2]

		proto := protocol.ID(protoOpt)

		listen, err := ma.NewMultiaddr(listenOpt)
		if err != nil {
			return err
		}

		targets, err := parseIpfsAddr(targetOpt)
		if err != nil {
			return err
		}

		allowCustom, _ := req.Options[allowCustomProtocolOptionName].(bool)

		if !allowCustom && !strings.HasPrefix(string(proto), P2PProtoPrefix) {
			return errors.New("protocol name must be within '" + P2PProtoPrefix + "' namespace")
		}

		return forwardLocal(n.Context(), n.P2P, n.Peerstore, proto, listen, targets)
	},
}

// parseIpfsAddr is a function that takes in addr string and return ipfsAddrs
func parseIpfsAddr(addr string) ([]ipfsaddr.IPFSAddr, error) {
	mutiladdr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	if _, err := mutiladdr.ValueForProtocol(ma.P_IPFS); err == nil {
		iaddrs := make([]ipfsaddr.IPFSAddr, 1)
		iaddrs[0], err = ipfsaddr.ParseMultiaddr(mutiladdr)
		if err != nil {
			return nil, err
		}
		return iaddrs, nil
	}
	// resolve mutiladdr whose protocol is not ma.P_IPFS
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	addrs, err := madns.Resolve(ctx, mutiladdr)
	cancel()
	if len(addrs) == 0 {
		return nil, errors.New("fail to resolve the multiaddr:" + mutiladdr.String())
	}
	iaddrs := make([]ipfsaddr.IPFSAddr, len(addrs))
	for i, addr := range addrs {
		iaddrs[i], err = ipfsaddr.ParseMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return iaddrs, nil
}

var p2pListenCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create libp2p service",
		ShortDescription: `
Create libp2p service and forward connections made to <target-address>.

<protocol> specifies the libp2p handler name. It must be prefixed with '` + P2PProtoPrefix + `'.

Example:
  ipfs p2p listen ` + P2PProtoPrefix + `myproto /ip4/127.0.0.1/tcp/1234
    - Forward connections to 'myproto' libp2p service to 127.0.0.1:1234

`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("protocol", true, false, "Protocol name."),
		cmdkit.StringArg("target-address", true, false, "Target endpoint."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(allowCustomProtocolOptionName, "Don't require /x/ prefix"),
		cmdkit.BoolOption(reportPeerIDOptionName, "r", "Send remote base58 peerid to target when a new connection is established"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		protoOpt := req.Arguments[0]
		targetOpt := req.Arguments[1]

		proto := protocol.ID(protoOpt)

		target, err := ma.NewMultiaddr(targetOpt)
		if err != nil {
			return err
		}

		// port can't be 0
		if err := checkPort(target); err != nil {
			return err
		}

		allowCustom, _ := req.Options[allowCustomProtocolOptionName].(bool)
		reportPeerID, _ := req.Options[reportPeerIDOptionName].(bool)

		if !allowCustom && !strings.HasPrefix(string(proto), P2PProtoPrefix) {
			return errors.New("protocol name must be within '" + P2PProtoPrefix + "' namespace")
		}

		_, err = n.P2P.ForwardRemote(n.Context(), proto, target, reportPeerID)
		return err
	},
}

// checkPort checks whether target multiaddr contains tcp or udp protocol
// and whether the port is equal to 0
func checkPort(target ma.Multiaddr) error {
	// get tcp or udp port from multiaddr
	getPort := func() (string, error) {
		sport, _ := target.ValueForProtocol(ma.P_TCP)
		if sport != "" {
			return sport, nil
		}

		sport, _ = target.ValueForProtocol(ma.P_UDP)
		if sport != "" {
			return sport, nil
		}
		return "", fmt.Errorf("address does not contain tcp or udp protocol")
	}

	sport, err := getPort()
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(sport)
	if err != nil {
		return err
	}

	if port == 0 {
		return fmt.Errorf("port can not be 0")
	}

	return nil
}

// forwardLocal forwards local connections to a libp2p service
func forwardLocal(ctx context.Context, p *p2p.P2P, ps pstore.Peerstore, proto protocol.ID, bindAddr ma.Multiaddr, addrs []ipfsaddr.IPFSAddr) error {
	for _, addr := range addrs {
		ps.AddAddr(addr.ID(), addr.Multiaddr(), pstore.TempAddrTTL)
	}
	// TODO: return some info
	// the length of the addrs must large than 0
	// peerIDs in addr must be the same and choose addr[0] to connect
	_, err := p.ForwardLocal(ctx, addrs[0].ID(), proto, bindAddr)
	return err
}

const (
	p2pHeadersOptionName = "headers"
)

var p2pLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List active p2p listeners.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(p2pHeadersOptionName, "v", "Print table headers (Protocol, Listen, Target)."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		output := &P2PLsOutput{}

		n.P2P.ListenersLocal.Lock()
		for _, listener := range n.P2P.ListenersLocal.Listeners {
			output.Listeners = append(output.Listeners, P2PListenerInfoOutput{
				Protocol:      string(listener.Protocol()),
				ListenAddress: listener.ListenAddress().String(),
				TargetAddress: listener.TargetAddress().String(),
			})
		}
		n.P2P.ListenersLocal.Unlock()

		n.P2P.ListenersP2P.Lock()
		for _, listener := range n.P2P.ListenersP2P.Listeners {
			output.Listeners = append(output.Listeners, P2PListenerInfoOutput{
				Protocol:      string(listener.Protocol()),
				ListenAddress: listener.ListenAddress().String(),
				TargetAddress: listener.TargetAddress().String(),
			})
		}
		n.P2P.ListenersP2P.Unlock()

		return cmds.EmitOnce(res, output)
	},
	Type: P2PLsOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *P2PLsOutput) error {
			headers, _ := req.Options[p2pHeadersOptionName].(bool)
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			for _, listener := range out.Listeners {
				if headers {
					fmt.Fprintln(tw, "Protocol\tListen Address\tTarget Address")
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\n", listener.Protocol, listener.ListenAddress, listener.TargetAddress)
			}
			tw.Flush()

			return nil
		}),
	},
}

const (
	p2pAllOptionName           = "all"
	p2pProtocolOptionName      = "protocol"
	p2pListenAddressOptionName = "listen-address"
	p2pTargetAddressOptionName = "target-address"
)

var p2pCloseCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Stop listening for new connections to forward.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(p2pAllOptionName, "a", "Close all listeners."),
		cmdkit.StringOption(p2pProtocolOptionName, "p", "Match protocol name"),
		cmdkit.StringOption(p2pListenAddressOptionName, "l", "Match listen address"),
		cmdkit.StringOption(p2pTargetAddressOptionName, "t", "Match target address"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		closeAll, _ := req.Options[p2pAllOptionName].(bool)
		protoOpt, p := req.Options[p2pProtocolOptionName].(string)
		listenOpt, l := req.Options[p2pListenAddressOptionName].(string)
		targetOpt, t := req.Options[p2pTargetAddressOptionName].(string)

		proto := protocol.ID(protoOpt)

		listen, err := ma.NewMultiaddr(listenOpt)
		if err != nil {
			return err
		}

		target, err := ma.NewMultiaddr(targetOpt)
		if err != nil {
			return err
		}

		if !(closeAll || p || l || t) {
			return errors.New("no matching options given")
		}

		if closeAll && (p || l || t) {
			return errors.New("can't combine --all with other matching options")
		}

		match := func(listener p2p.Listener) bool {
			if closeAll {
				return true
			}
			if p && proto != listener.Protocol() {
				return false
			}
			if l && !listen.Equal(listener.ListenAddress()) {
				return false
			}
			if t && !target.Equal(listener.TargetAddress()) {
				return false
			}
			return true
		}

		done := n.P2P.ListenersLocal.Close(match)
		done += n.P2P.ListenersP2P.Close(match)

		return cmds.EmitOnce(res, done)
	},
	Type: int(0),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out int) error {
			fmt.Fprintf(w, "Closed %d stream(s)\n", out)
			return nil
		}),
	},
}

///////
// Stream
//

// p2pStreamCmd is the 'ipfs p2p stream' command
var p2pStreamCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "P2P stream management.",
		ShortDescription: "Create and manage p2p streams",
	},

	Subcommands: map[string]*cmds.Command{
		"ls":    p2pStreamLsCmd,
		"close": p2pStreamCloseCmd,
	},
}

var p2pStreamLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List active p2p streams.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(p2pHeadersOptionName, "v", "Print table headers (ID, Protocol, Local, Remote)."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		output := &P2PStreamsOutput{}

		n.P2P.Streams.Lock()
		for id, s := range n.P2P.Streams.Streams {
			output.Streams = append(output.Streams, P2PStreamInfoOutput{
				HandlerID: strconv.FormatUint(id, 10),

				Protocol: string(s.Protocol),

				OriginAddress: s.OriginAddr.String(),
				TargetAddress: s.TargetAddr.String(),
			})
		}
		n.P2P.Streams.Unlock()

		return cmds.EmitOnce(res, output)
	},
	Type: P2PStreamsOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *P2PStreamsOutput) error {
			headers, _ := req.Options[p2pHeadersOptionName].(bool)
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			for _, stream := range out.Streams {
				if headers {
					fmt.Fprintln(tw, "ID\tProtocol\tOrigin\tTarget")
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", stream.HandlerID, stream.Protocol, stream.OriginAddress, stream.TargetAddress)
			}
			tw.Flush()

			return nil
		}),
	},
}

var p2pStreamCloseCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Close active p2p stream.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("id", false, false, "Stream identifier"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(p2pAllOptionName, "a", "Close all streams."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := p2pGetNode(env)
		if err != nil {
			return err
		}

		closeAll, _ := req.Options[p2pAllOptionName].(bool)
		var handlerID uint64

		if !closeAll {
			if len(req.Arguments) == 0 {
				return errors.New("no id specified")
			}

			handlerID, err = strconv.ParseUint(req.Arguments[0], 10, 64)
			if err != nil {
				return err
			}
		}

		toClose := make([]*p2p.Stream, 0, 1)
		n.P2P.Streams.Lock()
		for id, stream := range n.P2P.Streams.Streams {
			if !closeAll && handlerID != id {
				continue
			}
			toClose = append(toClose, stream)
			if !closeAll {
				break
			}
		}
		n.P2P.Streams.Unlock()

		for _, s := range toClose {
			n.P2P.Streams.Reset(s)
		}

		return nil
	},
}

func p2pGetNode(env cmds.Environment) (*core.IpfsNode, error) {
	nd, err := cmdenv.GetNode(env)
	if err != nil {
		return nil, err
	}

	config, err := nd.Repo.Config()
	if err != nil {
		return nil, err
	}

	if !config.Experimental.Libp2pStreamMounting {
		return nil, errors.New("libp2p stream mounting not enabled")
	}

	if !nd.OnlineMode() {
		return nil, ErrNotOnline
	}

	return nd, nil
}
