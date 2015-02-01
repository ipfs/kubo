package commands

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	cmds "github.com/jbenet/go-ipfs/commands"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	iaddr "github.com/jbenet/go-ipfs/util/ipfsaddr"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

type stringList struct {
	Strings []string
}

var SwarmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "swarm inspection tool",
		Synopsis: `
ipfs swarm peers                - List peers with open connections
ipfs swarm connect <address>    - Open connection to a given address
ipfs swarm disconnect <address> - Close connection to a given address
`,
		ShortDescription: `
ipfs swarm is a tool to manipulate the network swarm. The swarm is the
component that opens, listens for, and maintains connections to other
ipfs peers in the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"peers":      swarmPeersCmd,
		"connect":    swarmConnectCmd,
		"disconnect": swarmDisconnectCmd,
	},
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List peers with open connections",
		ShortDescription: `
ipfs swarm peers lists the set of peers this node is connected to.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {

		log.Debug("ipfs swarm peers")
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		conns := n.PeerHost.Network().Conns()
		addrs := make([]string, len(conns))
		for i, c := range conns {
			pid := c.RemotePeer()
			addr := c.RemoteMultiaddr()
			addrs[i] = fmt.Sprintf("%s/ipfs/%s", addr, pid.Pretty())
		}

		sort.Sort(sort.StringSlice(addrs))
		res.SetOutput(&stringList{addrs})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open connection to a given address",
		ShortDescription: `
'ipfs swarm connect' opens a connection to a peer address. The address format
is an ipfs multiaddr:

ipfs swarm connect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := context.TODO()

		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		addrs := req.Arguments()

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		peers, err := peersWithAddresses(n.Peerstore, addrs)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output := make([]string, len(peers))
		for i, p := range peers {
			output[i] = "connect " + p.Pretty()

			err := n.PeerHost.Connect(ctx, peer.PeerInfo{ID: p})
			if err != nil {
				output[i] += " failure: " + err.Error()
			} else {
				output[i] += " success"
			}
		}

		res.SetOutput(&stringList{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var swarmDisconnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Close connection to a given address",
		ShortDescription: `
'ipfs swarm disconnect' closes a connection to a peer address. The address format
is an ipfs multiaddr:

ipfs swarm disconnect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		addrs := req.Arguments()

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		iaddrs, err := parseAddresses(addrs)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output := make([]string, len(iaddrs))
		for i, addr := range iaddrs {
			taddr := addr.Transport()
			output[i] = "disconnect " + addr.ID().Pretty()

			found := false
			conns := n.PeerHost.Network().ConnsToPeer(addr.ID())
			for _, conn := range conns {
				if !conn.RemoteMultiaddr().Equal(taddr) {
					log.Error("it's not", conn.RemoteMultiaddr(), taddr)
					continue
				}

				if err := conn.Close(); err != nil {
					output[i] += " failure: " + err.Error()
				} else {
					output[i] += " success"
				}
				found = true
				break
			}

			if !found {
				output[i] += " failure: conn not found"
			}
		}
		res.SetOutput(&stringList{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

func stringListMarshaler(res cmds.Response) (io.Reader, error) {
	list, ok := res.Output().(*stringList)
	if !ok {
		return nil, errors.New("failed to cast []string")
	}

	var buf bytes.Buffer
	for _, s := range list.Strings {
		buf.WriteString(s)
		buf.WriteString("\n")
	}
	return &buf, nil
}

// parseAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns slices of multiaddrs and peerids.
func parseAddresses(addrs []string) (iaddrs []iaddr.IPFSAddr, err error) {
	iaddrs = make([]iaddr.IPFSAddr, len(addrs))
	for i, saddr := range addrs {
		iaddrs[i], err = iaddr.ParseString(saddr)
		if err != nil {
			return nil, cmds.ClientError("invalid peer address: " + err.Error())
		}
	}
	return
}

// peersWithAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns a slice of properly constructed peers
func peersWithAddresses(ps peer.Peerstore, addrs []string) (pids []peer.ID, err error) {
	iaddrs, err := parseAddresses(addrs)
	if err != nil {
		return nil, err
	}

	for _, iaddr := range iaddrs {
		pids = append(pids, iaddr.ID())
		ps.AddAddress(iaddr.ID(), iaddr.Multiaddr())
	}
	return pids, nil
}
