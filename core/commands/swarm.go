package commands

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"

	cmds "github.com/jbenet/go-ipfs/commands"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
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
			addrs[i] = fmt.Sprintf("%s/%s", addr, pid.Pretty())
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
		Tagline: "Open connection to a given peer",
		ShortDescription: `
'ipfs swarm connect' opens a connection to a peer address. The address format
is an ipfs multiaddr:

ipfs swarm connect /ip4/104.131.131.82/tcp/4001/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := context.TODO()

		log.Debug("ipfs swarm connect")
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
		id, err := peer.IDB58Decode(path.Base(addr))
		if err != nil {
			return nil, nil, err
		}
		pids[i] = id
		maddrs[i] = a
	}
	return
}

// peersWithAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns a slice of properly constructed peers
func peersWithAddresses(ps peer.Peerstore, addrs []string) ([]peer.ID, error) {
	maddrs, pids, err := splitAddresses(addrs)
	if err != nil {
		return nil, err
	}

	for i, p := range pids {
		ps.AddAddress(p, maddrs[i])
	}
	return pids, nil
}
