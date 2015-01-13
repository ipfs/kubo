package commands

import (
	"bytes"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	repo "github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/repo/fsrepo"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

// DefaultBootstrapAddresses are the hardcoded bootstrap addresses
// for ipfs. they are nodes run by the ipfs team. docs on these later.
// As with all p2p networks, bootstrap is an important security concern.
//
// Note: this is here -- and not inside cmd/ipfs/init.go -- because of an
// import dependency issue. TODO: move this into a config/default/ package.
var DefaultBootstrapAddresses = []string{
	"/ip4/104.131.131.82/tcp/4001/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",  // mars.i.ipfs.io
	"/ip4/104.236.176.52/tcp/4001/QmSoLnSGccFuZQJzRadHn95W2CrSFmZuTdDWP8HXaHca9z",  // neptune (to be neptune.i.ipfs.io)
	"/ip4/104.236.179.241/tcp/4001/QmSoLpPVmHKQ4XTPdz8tjDFgdeRFkpV8JgYq8JVJ69RrZm", // pluto (to be pluto.i.ipfs.io)
	"/ip4/162.243.248.213/tcp/4001/QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm", // uranus (to be uranus.i.ipfs.io)
	"/ip4/128.199.219.111/tcp/4001/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu", // saturn (to be saturn.i.ipfs.io)
}

type BootstrapOutput struct {
	Peers []config.BootstrapPeer
}

var peerOptionDesc = "A peer to add to the bootstrap list (in the format '<multiaddr>/<peerID>')"

var BootstrapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show or edit the list of bootstrap peers",
		Synopsis: `
ipfs bootstrap list             - Show peers in the bootstrap list
ipfs bootstrap add <peer>...    - Add peers to the bootstrap list
ipfs bootstrap rm <peer>... - Removes peers from the bootstrap list
`,
		ShortDescription: `
Running 'ipfs bootstrap' with no arguments will run 'ipfs bootstrap list'.
` + bootstrapSecurityWarning,
	},

	Run:        bootstrapListCmd.Run,
	Marshalers: bootstrapListCmd.Marshalers,
	Type:       bootstrapListCmd.Type,

	Subcommands: map[string]*cmds.Command{
		"list": bootstrapListCmd,
		"add":  bootstrapAddCmd,
		"rm":   bootstrapRemoveCmd,
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
		cmds.StringArg("peer", false, true, peerOptionDesc),
	},

	Options: []cmds.Option{
		cmds.BoolOption("default", "add default bootstrap nodes"),
	},

	Run: func(req cmds.Request) (interface{}, error) {
		inputPeers, err := config.ParseBootstrapPeers(req.Arguments())
		if err != nil {
			return nil, err
		}

		r := fsrepo.At(req.Context().ConfigRoot)
		if err := r.Open(); err != nil {
			return nil, err
		}
		defer r.Close()
		cfg := r.Config()

		deflt, _, err := req.Option("default").Bool()
		if err != nil {
			return nil, err
		}

		if deflt {
			// parse separately for meaningful, correct error.
			defltPeers, err := DefaultBootstrapPeers()
			if err != nil {
				return nil, err
			}

			inputPeers = append(inputPeers, defltPeers...)
		}

		added, err := bootstrapAdd(r, cfg, inputPeers)
		if err != nil {
			return nil, err
		}

		if len(inputPeers) == 0 {
			return nil, cmds.ClientError("no bootstrap peers to add")
		}

		return &BootstrapOutput{added}, nil
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, ok := res.Output().(*BootstrapOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			var buf bytes.Buffer
			err := bootstrapWritePeers(&buf, "added ", v.Peers)
			return &buf, err
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
		cmds.StringArg("peer", false, true, peerOptionDesc),
	},
	Options: []cmds.Option{
		cmds.BoolOption("all", "Remove all bootstrap peers."),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		input, err := config.ParseBootstrapPeers(req.Arguments())
		if err != nil {
			return nil, err
		}

		r := fsrepo.At(req.Context().ConfigRoot)
		if err := r.Open(); err != nil {
			return nil, err
		}
		defer r.Close()
		cfg := r.Config()

		all, _, err := req.Option("all").Bool()
		if err != nil {
			return nil, err
		}

		var removed []config.BootstrapPeer
		if all {
			removed, err = bootstrapRemoveAll(r, cfg)
		} else {
			removed, err = bootstrapRemove(r, cfg, input)
		}
		if err != nil {
			return nil, err
		}

		return &BootstrapOutput{removed}, nil
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, ok := res.Output().(*BootstrapOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			var buf bytes.Buffer
			err := bootstrapWritePeers(&buf, "removed ", v.Peers)
			return &buf, err
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
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: bootstrapMarshaler,
	},
}

func bootstrapMarshaler(res cmds.Response) (io.Reader, error) {
	v, ok := res.Output().(*BootstrapOutput)
	if !ok {
		return nil, u.ErrCast()
	}

	var buf bytes.Buffer
	err := bootstrapWritePeers(&buf, "", v.Peers)
	return &buf, err
}

func bootstrapWritePeers(w io.Writer, prefix string, peers []config.BootstrapPeer) error {

	for _, peer := range peers {
		s := prefix + peer.Address + "/" + peer.PeerID + "\n"
		_, err := w.Write([]byte(s))
		if err != nil {
			return err
		}
	}
	return nil
}

func bootstrapAdd(r repo.Repo, cfg *config.Config, peers []config.BootstrapPeer) ([]config.BootstrapPeer, error) {
	added := make([]config.BootstrapPeer, 0, len(peers))

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

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return added, nil
}

func bootstrapRemove(r repo.Repo, cfg *config.Config, toRemove []config.BootstrapPeer) ([]config.BootstrapPeer, error) {
	removed := make([]config.BootstrapPeer, 0, len(toRemove))
	keep := make([]config.BootstrapPeer, 0, len(cfg.Bootstrap))

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

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return removed, nil
}

func bootstrapRemoveAll(r repo.Repo, cfg *config.Config) ([]config.BootstrapPeer, error) {
	removed := make([]config.BootstrapPeer, len(cfg.Bootstrap))
	copy(removed, cfg.Bootstrap)

	cfg.Bootstrap = nil
	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return removed, nil
}

// DefaultBootstrapPeers returns the (parsed) set of default bootstrap peers.
// if it fails, it returns a meaningful error for the user.
// This is here (and not inside cmd/ipfs/init) because of module dependency problems.
func DefaultBootstrapPeers() ([]config.BootstrapPeer, error) {
	ps, err := config.ParseBootstrapPeers(DefaultBootstrapAddresses)
	if err != nil {
		return nil, errors.Errorf(`failed to parse hardcoded bootstrap peers: %s
This is a problem with the ipfs codebase. Please report it to the dev team.`, err)
	}
	return ps, nil
}

const bootstrapSecurityWarning = `
SECURITY WARNING:

The bootstrap command manipulates the "bootstrap list", which contains
the addresses of bootstrap nodes. These are the *trusted peers* from
which to learn about other peers in the network. Only edit this list
if you understand the risks of adding or removing nodes from this list.

`
