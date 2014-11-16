package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
)

type IdOutput struct {
	ID              string
	PublicKey       string
	Addresses       []string
	AgentVersion    string
	ProtocolVersion string
}

var idCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show IPFS Node IF info",
		ShortDescription: `Prints out information about the specified peer,
		if no peer is specified, prints out local peers info.`,
	},
	Arguments: nil,
	Run: func(req cmds.Request) (interface{}, error) {
		node, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		if len(req.Arguments()) == 0 {
			return printPeer(node.Identity)
		}

		pid, ok := req.Arguments()[0].(string)
		if !ok {
			return nil, errors.New("Improperly formatted peer id")
		}

		id := peer.ID(b58.Decode(pid))
		ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
		p, err := node.Routing.FindPeer(ctx, id)
		if err == kb.ErrLookupFailure {
			return nil, errors.New(`ID command fails when run without daemon, we are working to fix this
		In the meantime, please run the daemon if you want to use 'ipfs id'`)
		}
		if err != nil {
			return nil, err
		}
		return printPeer(p)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			val, ok := res.Output().(*IdOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			return json.MarshalIndent(val, "", "\t")
		},
	},
	Type: &IdOutput{},
}

func printPeer(p peer.Peer) (interface{}, error) {
	if p == nil {
		return nil, errors.New("Attempted to print nil peer!")
	}
	info := new(IdOutput)

	info.ID = p.ID().String()
	if p.PubKey() == nil {
		return nil, errors.New(`peer publickey not populated on offline runs,
		please run the daemon to use ipfs id!`)
	}
	pkb, err := p.PubKey().Bytes()
	if err != nil {
		return nil, err
	}
	info.PublicKey = base64.StdEncoding.EncodeToString(pkb)
	for _, a := range p.Addresses() {
		info.Addresses = append(info.Addresses, a.String())
	}

	agent, protocol := p.GetVersions()
	info.AgentVersion = agent
	info.ProtocolVersion = protocol

	return info, nil
}
