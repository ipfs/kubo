package commands

import (
	"bytes"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

type stringList struct {
	Strings []string
}

var swarmCmd = &cmds.Command{
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
		"peers": swarmPeersCmd,
		// "connect": swarmConnectCmd,
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
