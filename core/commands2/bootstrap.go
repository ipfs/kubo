package commands

import (
	"bytes"
	"io"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
)

type BootstrapOutput struct {
	Peers []*config.BootstrapPeer
}

var peerOptionDesc = "A peer to add to the bootstrap list (in the format '<multiaddr>/<peerID>')"

var bootstrapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show or edit the list of bootstrap peers",
		Synopsis: `
ipfs bootstrap list             - Show peers in the bootstrap list
ipfs bootstrap add <peer>...    - Add peers to the bootstrap list
ipfs bootstrap remove <peer>... - Removes peers from the bootstrap list
`,
		ShortDescription: `
Running 'ipfs bootstrap' with no arguments will run 'ipfs bootstrap list'.
` + bootstrapSecurityWarning,
	},

	Run:        bootstrapListCmd.Run,
	Marshalers: bootstrapListCmd.Marshalers,
	Type:       bootstrapListCmd.Type,

	Subcommands: map[string]*cmds.Command{
		"list":   bootstrapListCmd,
		"add":    bootstrapAddCmd,
		"rm": bootstrapRemoveCmd,
	},
}

var bootstrapAddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add peers to the bootstrap list",
		ShortDescription: `Outputs a list of peers that were added (that weren't already
in the bootstrap list).
` + bootstrapSecurityWarning,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peer", true, true, peerOptionDesc),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		input, err := bootstrapInputToPeers(req.Arguments())
		if err != nil {
			return nil, err
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			return nil, err
		}

		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		added, err := bootstrapAdd(filename, cfg, input)
		if err != nil {
			return nil, err
		}

		return &BootstrapOutput{added}, nil
	},
	Type: &BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v, ok := res.Output().(*BootstrapOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			var buf bytes.Buffer
			err := bootstrapWritePeers(&buf, "added ", v.Peers)
			return buf.Bytes(), err
		},
	},
}

var bootstrapRemoveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Removes peers from the bootstrap list",
		ShortDescription: `Outputs the list of peers that were removed.
` + bootstrapSecurityWarning,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peer", true, true, peerOptionDesc),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		input, err := bootstrapInputToPeers(req.Arguments())
		if err != nil {
			return nil, err
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			return nil, err
		}

		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		removed, err := bootstrapRemove(filename, cfg, input)
		if err != nil {
			return nil, err
		}

		return &BootstrapOutput{removed}, nil
	},
	Type: &BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v, ok := res.Output().(*BootstrapOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			var buf bytes.Buffer
			err := bootstrapWritePeers(&buf, "removed ", v.Peers)
			return buf.Bytes(), err
		},
	},
}

var bootstrapListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show peers in the bootstrap list",
		ShortDescription: "Peers are output in the format '<multiaddr>/<peerID>'.",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		peers := cfg.Bootstrap
		return &BootstrapOutput{peers}, nil
	},
	Type: &BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: bootstrapMarshaler,
	},
}

func bootstrapMarshaler(res cmds.Response) ([]byte, error) {
	v, ok := res.Output().(*BootstrapOutput)
	if !ok {
		return nil, u.ErrCast()
	}

	var buf bytes.Buffer
	err := bootstrapWritePeers(&buf, "", v.Peers)
	return buf.Bytes(), err
}

func bootstrapWritePeers(w io.Writer, prefix string, peers []*config.BootstrapPeer) error {

	for _, peer := range peers {
		s := prefix + peer.Address + "/" + peer.PeerID + "\n"
		_, err := w.Write([]byte(s))
		if err != nil {
			return err
		}
	}
	return nil
}

func bootstrapInputToPeers(input []interface{}) ([]*config.BootstrapPeer, error) {
	inputAddrs := make([]string, len(input))
	for i, v := range input {
		addr, ok := v.(string)
		if !ok {
			return nil, u.ErrCast()
		}
		inputAddrs[i] = addr
	}

	split := func(addr string) (string, string) {
		idx := strings.LastIndex(addr, "/")
		if idx == -1 {
			return "", addr
		}
		return addr[:idx], addr[idx+1:]
	}

	peers := []*config.BootstrapPeer{}
	for _, addr := range inputAddrs {
		addrS, peeridS := split(addr)

		// make sure addrS parses as a multiaddr.
		if len(addrS) > 0 {
			maddr, err := ma.NewMultiaddr(addrS)
			if err != nil {
				return nil, err
			}

			addrS = maddr.String()
		}

		// make sure idS parses as a peer.ID
		_, err := mh.FromB58String(peeridS)
		if err != nil {
			return nil, err
		}

		// construct config entry
		peers = append(peers, &config.BootstrapPeer{
			Address: addrS,
			PeerID:  peeridS,
		})
	}
	return peers, nil
}

func bootstrapAdd(filename string, cfg *config.Config, peers []*config.BootstrapPeer) ([]*config.BootstrapPeer, error) {
	added := make([]*config.BootstrapPeer, 0, len(peers))

	for _, peer := range peers {
		duplicate := false
		for _, peer2 := range cfg.Bootstrap {
			if peer.Address == peer2.Address && peer.PeerID == peer2.PeerID {
				duplicate = true
				break
			}
		}

		if !duplicate {
			cfg.Bootstrap = append(cfg.Bootstrap, peer)
			added = append(added, peer)
		}
	}

	err := config.WriteConfigFile(filename, cfg)
	if err != nil {
		return nil, err
	}

	return added, nil
}

func bootstrapRemove(filename string, cfg *config.Config, toRemove []*config.BootstrapPeer) ([]*config.BootstrapPeer, error) {
	removed := make([]*config.BootstrapPeer, 0, len(toRemove))
	keep := make([]*config.BootstrapPeer, 0, len(cfg.Bootstrap))

	for _, peer := range cfg.Bootstrap {
		found := false
		for _, peer2 := range toRemove {
			if peer.Address == peer2.Address && peer.PeerID == peer2.PeerID {
				found = true
				removed = append(removed, peer)
				break
			}
		}

		if !found {
			keep = append(keep, peer)
		}
	}
	cfg.Bootstrap = keep

	err := config.WriteConfigFile(filename, cfg)
	if err != nil {
		return nil, err
	}

	return removed, nil
}

const bootstrapSecurityWarning = `
SECURITY WARNING:

The bootstrap command manipulates the "bootstrap list", which contains
the addresses of bootstrap nodes. These are the *trusted peers* from
which to learn about other peers in the network. Only edit this list
if you understand the risks of adding or removing nodes from this list.

`
