package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	kb "gx/ipfs/QmTxn7JEA8DiBvd9vVzErAzadHn6TwjCKTjjUfPyRH9wjZ/go-libp2p-kbucket"

	ic "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	"gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	u "gx/ipfs/QmZuY8aV7zbNXVy6DyN9SmnuH3o9nG852F4aTiSBpts8d1/go-ipfs-util"
	pstore "gx/ipfs/Qme1g4e3m2SmdiSGGU3vSWmUStwUjc5oECnEriaK9Xa1HU/go-libp2p-peerstore"
	identify "gx/ipfs/QmeWJwi61vii5g8zQUB9UGegfUbmhTKHgeDFP9XuSp5jZ4/go-libp2p/p2p/protocol/identify"
)

const offlineIdErrorMessage = `'ipfs id' currently cannot query information on remote
peers without a running daemon; we are working to fix this.
In the meantime, if you want to query remote peers using 'ipfs id',
please run the daemon:

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
		Tagline: "Show ipfs node id info.",
		ShortDescription: `
Prints out information about the specified peer.
If no peer is specified, prints out information for local peers.

'ipfs id' supports the format option for output with the following keys:
<id> : The peers id.
<aver>: Agent version.
<pver>: Protocol version.
<pubkey>: Public key.
<addrs>: Addresses (newline delimited).

EXAMPLE:

    ipfs id Qmece2RkXhsKe5CRooNisBTh4SK119KrXXGmoK6V3kb8aH -f="<addrs>\n"
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerid", false, false, "Peer.ID of node to look up."),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Optional output format."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var id peer.ID
		if len(req.Arguments()) > 0 {
			id = peer.ID(b58.Decode(req.Arguments()[0]))
			if len(id) == 0 {
				res.SetError(cmds.ClientError("Invalid peer id"), cmds.ErrClient)
				return
			}
		} else {
			id = node.Identity
		}

		if id == node.Identity {
			output, err := printSelf(node)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(output)
			return
		}

		// TODO handle offline mode with polymorphism instead of conditionals
		if !node.OnlineMode() {
			res.SetError(errors.New(offlineIdErrorMessage), cmds.ErrClient)
			return
		}

		p, err := node.Routing.FindPeer(req.Context(), id)
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
				output = strings.Replace(output, "<addrs>", strings.Join(val.Addresses, "\n"), -1)
				output = strings.Replace(output, "\\n", "\n", -1)
				output = strings.Replace(output, "\\t", "\t", -1)
				return strings.NewReader(output), nil
			} else {

				marshaled, err := json.MarshalIndent(val, "", "\t")
				if err != nil {
					return nil, err
				}
				marshaled = append(marshaled, byte('\n'))
				return bytes.NewReader(marshaled), nil
			}
		},
	},
	Type: IdOutput{},
}

func printPeer(ps pstore.Peerstore, p peer.ID) (interface{}, error) {
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
	info.ProtocolVersion = identify.LibP2PVersion
	info.AgentVersion = identify.ClientVersion
	return info, nil
}
