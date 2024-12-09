package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	ke "github.com/ipfs/kubo/core/commands/keyencode"
	kb "github.com/libp2p/go-libp2p-kbucket"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const offlineIDErrorMessage = "'ipfs id' cannot query information on remote peers without a running daemon; if you only want to convert --peerid-base, pass --offline option"

type IdOutput struct { // nolint
	ID           string
	PublicKey    string
	Addresses    []string
	AgentVersion string
	Protocols    []protocol.ID
}

const (
	formatOptionName   = "format"
	idFormatOptionName = "peerid-base"
)

var IDCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show IPFS node id info.",
		ShortDescription: `
Prints out information about the specified peer.
If no peer is specified, prints out information for local peers.

'ipfs id' supports the format option for output with the following keys:
<id> : The peers id.
<aver>: Agent version.
<pver>: Protocol version.
<pubkey>: Public key.
<addrs>: Addresses (newline delimited).
<protocols>: Libp2p Protocol registrations (newline delimited).

EXAMPLE:

    ipfs id Qmece2RkXhsKe5CRooNisBTh4SK119KrXXGmoK6V3kb8aH -f="<addrs>\n"
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerid", false, false, "Peer.ID of node to look up."),
	},
	Options: []cmds.Option{
		cmds.StringOption(formatOptionName, "f", "Optional output format."),
		cmds.StringOption(idFormatOptionName, "Encoding used for peer IDs: Can either be a multibase encoded CID or a base58btc encoded multihash. Takes {b58mh|base36|k|base32|b...}.").WithDefault("b58mh"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		keyEnc, err := ke.KeyEncoderFromString(req.Options[idFormatOptionName].(string))
		if err != nil {
			return err
		}

		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		var id peer.ID
		if len(req.Arguments) > 0 {
			var err error
			id, err = peer.Decode(req.Arguments[0])
			if err != nil {
				return errors.New("invalid peer id")
			}
		} else {
			id = n.Identity
		}

		if id == n.Identity {
			output, err := printSelf(keyEnc, n)
			if err != nil {
				return err
			}
			return cmds.EmitOnce(res, output)
		}

		offline, _ := req.Options[OfflineOption].(bool)
		if !offline && !n.IsOnline {
			return errors.New(offlineIDErrorMessage)
		}

		if !offline {
			// We need to actually connect to run identify.
			err = n.PeerHost.Connect(req.Context, peer.AddrInfo{ID: id})
			switch err {
			case nil:
			case kb.ErrLookupFailure:
				return errors.New(offlineIDErrorMessage)
			default:
				return err
			}
		}

		output, err := printPeer(keyEnc, n.Peerstore, id)
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *IdOutput) error {
			format, found := req.Options[formatOptionName].(string)
			if found {
				output := format
				output = strings.Replace(output, "<id>", out.ID, -1)
				output = strings.Replace(output, "<aver>", out.AgentVersion, -1)
				output = strings.Replace(output, "<pubkey>", out.PublicKey, -1)
				output = strings.Replace(output, "<addrs>", strings.Join(out.Addresses, "\n"), -1)
				output = strings.Replace(output, "<protocols>", strings.Join(protocol.ConvertToStrings(out.Protocols), "\n"), -1)
				output = strings.Replace(output, "\\n", "\n", -1)
				output = strings.Replace(output, "\\t", "\t", -1)
				fmt.Fprint(w, output)
			} else {
				marshaled, err := json.MarshalIndent(out, "", "\t")
				if err != nil {
					return err
				}
				marshaled = append(marshaled, byte('\n'))
				fmt.Fprintln(w, string(marshaled))
			}
			return nil
		}),
	},
	Type: IdOutput{},
}

func printPeer(keyEnc ke.KeyEncoder, ps pstore.Peerstore, p peer.ID) (interface{}, error) {
	if p == "" {
		return nil, errors.New("attempted to print nil peer")
	}

	info := new(IdOutput)
	info.ID = keyEnc.FormatID(p)

	if pk := ps.PubKey(p); pk != nil {
		pkb, err := ic.MarshalPublicKey(pk)
		if err != nil {
			return nil, err
		}
		info.PublicKey = base64.StdEncoding.EncodeToString(pkb)
	}

	addrInfo := ps.PeerInfo(p)
	addrs, err := peer.AddrInfoToP2pAddrs(&addrInfo)
	if err != nil {
		return nil, err
	}

	for _, a := range addrs {
		info.Addresses = append(info.Addresses, a.String())
	}
	sort.Strings(info.Addresses)

	protocols, _ := ps.GetProtocols(p) // don't care about errors here.
	info.Protocols = append(info.Protocols, protocols...)
	sort.Slice(info.Protocols, func(i, j int) bool { return info.Protocols[i] < info.Protocols[j] })

	if v, err := ps.Get(p, "AgentVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.AgentVersion = vs
		}
	}

	return info, nil
}

// printing self is special cased as we get values differently.
func printSelf(keyEnc ke.KeyEncoder, node *core.IpfsNode) (interface{}, error) {
	info := new(IdOutput)
	info.ID = keyEnc.FormatID(node.Identity)

	pk := node.PrivateKey.GetPublic()
	pkb, err := ic.MarshalPublicKey(pk)
	if err != nil {
		return nil, err
	}
	info.PublicKey = base64.StdEncoding.EncodeToString(pkb)

	if node.PeerHost != nil {
		addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(node.PeerHost))
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			info.Addresses = append(info.Addresses, a.String())
		}
		sort.Strings(info.Addresses)
		info.Protocols = node.PeerHost.Mux().Protocols()
		sort.Slice(info.Protocols, func(i, j int) bool { return info.Protocols[i] < info.Protocols[j] })
	}
	info.AgentVersion = version.GetUserAgentVersion()
	return info, nil
}
