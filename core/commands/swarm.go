package commands

import (
	"bytes"
	"fmt"
	"path"

	cmds "github.com/jbenet/go-ipfs/commands"
	peer "github.com/jbenet/go-ipfs/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

type stringList struct {
	Strings []string
}

var SwarmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "swarm inspection tool",
		Synopsis: `
ipfs swarm peers             - List peers with open connections
ipfs swarm connect <address> - Open connection to a given peer
`,
		ShortDescription: `
ipfs swarm is a tool to manipulate the network swarm. The swarm is the
component that opens, listens for, and maintains connections to other
ipfs peers in the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"peers":   swarmPeersCmd,
		"connect": swarmConnectCmd,
	},
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List peers with open connections",
		ShortDescription: `
ipfs swarm peers lists the set of peers this node is connected to.
`,
	},
	Run: func(req cmds.Request) (interface{}, error) {

		log.Debug("ipfs swarm peers")
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		if n.Network == nil {
			return nil, errNotOnline
		}

		conns := n.Network.GetConnections()
		addrs := make([]string, len(conns))
		for i, c := range conns {
			pid := c.RemotePeer().ID()
			addr := c.RemoteMultiaddr()
			addrs[i] = fmt.Sprintf("%s/%s", addr, pid)
		}

		return &stringList{addrs}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: &stringList{},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open connection to a given peer",
		ShortDescription: `
'ipfs swarm connect' opens a connection to a peer address. The address format
is an ipfs multiaddr:

ipfs swarm connect /ip4/104.131.131.82/tcp/4001/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		ctx := context.TODO()

		log.Debug("ipfs swarm connect")
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		addrs := req.Arguments()

		if n.Network == nil {
			return nil, errNotOnline
		}

		peers, err := peersWithAddresses(n.Peerstore, addrs)
		if err != nil {
			return nil, err
		}

		output := make([]string, len(peers))
		for i, p := range peers {
			output[i] = "connect " + p.ID().String()

			err := n.Network.DialPeer(ctx, p)
			if err != nil {
				output[i] += " failure: " + err.Error()
			} else {
				output[i] += " success"
			}
		}

		return &stringList{output}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: &stringList{},
}

func stringListMarshaler(res cmds.Response) ([]byte, error) {
	list, ok := res.Output().(*stringList)
	if !ok {
		return nil, errors.New("failed to cast []string")
	}

	var buf bytes.Buffer
	for _, s := range list.Strings {
		buf.Write([]byte(s))
		buf.Write([]byte("\n"))
	}
	return buf.Bytes(), nil
}

// splitAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns slices of multiaddrs and peerids.
func splitAddresses(addrs []string) (maddrs []ma.Multiaddr, pids []peer.ID, err error) {

	maddrs = make([]ma.Multiaddr, len(addrs))
	pids = make([]peer.ID, len(addrs))
	for i, addr := range addrs {
		a, err := ma.NewMultiaddr(path.Dir(addr))
		if err != nil {
			return nil, nil, cmds.ClientError("invalid peer address: " + err.Error())
		}
		maddrs[i] = a
		pids[i] = peer.DecodePrettyID(path.Base(addr))
	}
	return
}

// peersWithAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns a slice of properly constructed peers
func peersWithAddresses(ps peer.Peerstore, addrs []string) ([]peer.Peer, error) {
	maddrs, pids, err := splitAddresses(addrs)
	if err != nil {
		return nil, err
	}

	peers := make([]peer.Peer, len(pids))
	for i, pid := range pids {
		p, err := ps.Get(pid)
		if err != nil {
			return nil, err
		}

		p.AddAddress(maddrs[i])
		peers[i] = p
	}
	return peers, nil
}
