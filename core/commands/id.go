package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"

	cmds "github.com/jbenet/go-ipfs/commands"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	"github.com/jbenet/go-ipfs/p2p/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
)

const offlineIdErrorMessage = `ID command fails when run without daemon, we are working to fix this.
In the meantime, please run the daemon if you want to use 'ipfs id':

    ipfs daemon &
    ipfs id QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`

type IdOutput struct {
	ID              string
	PublicKey       string
	Addresses       []string
	AgentVersion    string
	ProtocolVersion string
}

var IDCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show IPFS Node ID info",
		ShortDescription: `
Prints out information about the specified peer,
if no peer is specified, prints out local peers info.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerid", false, false, "peer.ID of node to look up"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		node, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		if len(req.Arguments()) == 0 {
			return printPeer(node.Peerstore, node.Identity)
		}

		pid := req.Arguments()[0]

		id := peer.ID(b58.Decode(pid))
		if len(id) == 0 {
			return nil, cmds.ClientError("Invalid peer id")
		}

		ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
		// TODO handle offline mode with polymorphism instead of conditionals
		if !node.OnlineMode() {
			return nil, errors.New(offlineIdErrorMessage)
		}

		p, err := node.Routing.FindPeer(ctx, id)
		if err == kb.ErrLookupFailure {
			return nil, errors.New(offlineIdErrorMessage)
		}
		if err != nil {
			return nil, err
		}
		return printPeer(node.Peerstore, p.ID)
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

func printPeer(ps peer.Peerstore, p peer.ID) (interface{}, error) {
	if p == "" {
		return nil, errors.New("Attempted to print nil peer!")
	}

	info := new(IdOutput)
	info.ID = p.Pretty()

	if pk := ps.PubKey(p); pk != nil {
		pkb, err := ic.MarshalPublicKey(pk)
		if err != nil {
			return nil, err
		}
		info.PublicKey = base64.StdEncoding.EncodeToString(pkb)
	}

	for _, a := range ps.Addresses(p) {
		info.Addresses = append(info.Addresses, a.String())
	}

	if v, err := ps.Get(p, "ProtocolVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.AgentVersion = vs
		}
	}
	if v, err := ps.Get(p, "AgentVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.ProtocolVersion = vs
		}
	}

	return info, nil
}
