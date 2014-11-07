package commands

import (
	"errors"
	"fmt"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	//peer "github.com/jbenet/go-ipfs/peer"
	//u "github.com/jbenet/go-ipfs/util"
)

type BootstrapOutput struct {
	Peers []*config.BootstrapPeer
}

var bootstrapCmd = &cmds.Command{
	Help: `ipfs bootstrap - show, or manipulate bootstrap node addresses

Running 'ipfs bootstrap' with no arguments will run 'ipfs bootstrap list'.

Commands:

  list               Show the boostrap list.
  add <address>      Add a node's address to the bootstrap list.
  remove <address>   Remove an address from the bootstrap list.

` + bootstrapSecurityWarning,
	Run:         bootstrapListCmd.Run,
	Marshallers: bootstrapListCmd.Marshallers,
	Subcommands: map[string]*cmds.Command{
		"list":   bootstrapListCmd,
		"add":    bootstrapAddCmd,
		"remove": bootstrapRemoveCmd,
	},
}

var bootstrapAddCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"peer", cmds.ArgString, true, true},
	},
	Help: `ipfs bootstrap add - add addresses to the bootstrap list
` + bootstrapSecurityWarning,
	Run: func(res cmds.Response, req cmds.Request) {
		input, err := bootstrapInputToPeers(req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		added, err := bootstrapAdd(filename, req.Context().Config, input)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{added})
	},
	Type: &BootstrapOutput{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*BootstrapOutput)
			s := fmt.Sprintf("Added %v peers to the bootstrap list:\n", len(v.Peers))
			marshalled, err := bootstrapMarshaller(res)
			if err != nil {
				return nil, err
			}
			return append([]byte(s), marshalled...), nil
		},
	},
}

var bootstrapRemoveCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"peer", cmds.ArgString, true, true},
	},
	Help: `ipfs bootstrap remove - remove addresses from the bootstrap list
` + bootstrapSecurityWarning,
	Run: func(res cmds.Response, req cmds.Request) {
		input, err := bootstrapInputToPeers(req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		removed, err := bootstrapRemove(filename, req.Context().Config, input)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{removed})
	},
	Type: &BootstrapOutput{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*BootstrapOutput)
			s := fmt.Sprintf("Removed %v peers from the bootstrap list:\n", len(v.Peers))
			marshalled, err := bootstrapMarshaller(res)
			if err != nil {
				return nil, err
			}
			return append([]byte(s), marshalled...), nil
		},
	},
}

var bootstrapListCmd = &cmds.Command{
	Help: "ipfs bootstrap list - Show addresses in the bootstrap list",
	Run: func(res cmds.Response, req cmds.Request) {
		peers := req.Context().Config.Bootstrap
		res.SetOutput(&BootstrapOutput{peers})
	},
	Type: &BootstrapOutput{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: bootstrapMarshaller,
	},
}

func bootstrapMarshaller(res cmds.Response) ([]byte, error) {
	v := res.Output().(*BootstrapOutput)

	s := ""
	for _, peer := range v.Peers {
		s += fmt.Sprintf("%s/%s\n", peer.Address, peer.PeerID)
	}

	return []byte(s), nil
}

func bootstrapInputToPeers(input []interface{}) ([]*config.BootstrapPeer, error) {
	split := func(addr string) (string, string) {
		idx := strings.LastIndex(addr, "/")
		if idx == -1 {
			return "", addr
		}
		return addr[:idx], addr[idx+1:]
	}

	peers := []*config.BootstrapPeer{}
	for _, v := range input {
		addr, ok := v.(string)
		if !ok {
			return nil, errors.New("cast error")
		}

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
			if peer.Address == peer2.Address {
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

func bootstrapRemove(filename string, cfg *config.Config, peers []*config.BootstrapPeer) ([]*config.BootstrapPeer, error) {
	removed := make([]*config.BootstrapPeer, 0, len(peers))
	keep := make([]*config.BootstrapPeer, 0, len(cfg.Bootstrap))

	for _, peer := range cfg.Bootstrap {
		found := false
		for _, peer2 := range peers {
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
