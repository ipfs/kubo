package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	"github.com/jbenet/go-ipfs/p2p/peer"
	identify "github.com/jbenet/go-ipfs/p2p/protocol/identify"
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

ipfs id supports the format option for output with the following keys:
<id> : the peers id
<aver>: agent version
<pver>: protocol version
<pubkey>: public key
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerid", false, false, "peer.ID of node to look up").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("f", "format", "optional output format"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if len(req.Arguments()) == 0 {
			output, err := printSelf(node)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(output)
			return
		}

		pid := req.Arguments()[0]

		id := peer.ID(b58.Decode(pid))
		if len(id) == 0 {
			res.SetError(cmds.ClientError("Invalid peer id"), cmds.ErrClient)
			return
		}

		ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
		// TODO handle offline mode with polymorphism instead of conditionals
		if !node.OnlineMode() {
			res.SetError(errors.New(offlineIdErrorMessage), cmds.ErrClient)
			return
		}

		p, err := node.Routing.FindPeer(ctx, id)
		if err == kb.ErrLookupFailure {
			res.SetError(errors.New(offlineIdErrorMessage), cmds.ErrClient)
			return
		}
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output, err := printPeer(node.Peerstore, p.ID)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			val, ok := res.Output().(*IdOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			format, found, err := res.Request().Option("format").String()
			if err != nil {
				return nil, err
			}
			if found {
				output := format
				output = strings.Replace(output, "<id>", val.ID, -1)
				output = strings.Replace(output, "<aver>", val.AgentVersion, -1)
				output = strings.Replace(output, "<pver>", val.ProtocolVersion, -1)
				output = strings.Replace(output, "<pubkey>", val.PublicKey, -1)
				return strings.NewReader(output), nil
			} else {

				marshaled, err := json.MarshalIndent(val, "", "\t")
				if err != nil {
					return nil, err
				}
				return bytes.NewReader(marshaled), nil
			}
		},
	},
	Type: IdOutput{},
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

	for _, a := range ps.Addrs(p) {
		info.Addresses = append(info.Addresses, a.String())
	}

	if v, err := ps.Get(p, "ProtocolVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.ProtocolVersion = vs
		}
	}
	if v, err := ps.Get(p, "AgentVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.AgentVersion = vs
		}
	}

	return info, nil
}

// printing self is special cased as we get values differently.
func printSelf(node *core.IpfsNode) (interface{}, error) {
	info := new(IdOutput)
	info.ID = node.Identity.Pretty()

	if node.PrivateKey == nil {
		if err := node.LoadPrivateKey(); err != nil {
			return nil, err
		}
	}

	pk := node.PrivateKey.GetPublic()
	pkb, err := ic.MarshalPublicKey(pk)
	if err != nil {
		return nil, err
	}
	info.PublicKey = base64.StdEncoding.EncodeToString(pkb)

	if node.PeerHost != nil {
		for _, a := range node.PeerHost.Addrs() {
			s := a.String() + "/ipfs/" + info.ID
			info.Addresses = append(info.Addresses, s)
		}
	}
	info.ProtocolVersion = identify.IpfsVersion
	info.AgentVersion = identify.ClientVersion
	return info, nil
}
