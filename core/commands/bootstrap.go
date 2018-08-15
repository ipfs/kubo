package commands

import (
	"bytes"
	"errors"
	"io"
	"sort"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	repo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
)

type BootstrapOutput struct {
	Peers []string
}

var peerOptionDesc = "A peer to add to the bootstrap list (in the format '<multiaddr>/<peerID>')"

var BootstrapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show or edit the list of bootstrap peers.",
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
	Helptext: cmdkit.HelpText{
		Tagline: "Add peers to the bootstrap list.",
		ShortDescription: `Outputs a list of peers that were added (that weren't already
in the bootstrap list).
` + bootstrapSecurityWarning,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer", false, true, peerOptionDesc).EnableStdin(),
	},

	Options: []cmdkit.Option{
		cmdkit.BoolOption("default", "Add default bootstrap nodes. (Deprecated, use 'default' subcommand instead)"),
	},
	Subcommands: map[string]*cmds.Command{
		"default": bootstrapAddDefaultCmd,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		deflt, _, err := req.Option("default").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var inputPeers []config.BootstrapPeer
		if deflt {
			// parse separately for meaningful, correct error.
			defltPeers, err := config.DefaultBootstrapPeers()
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			inputPeers = defltPeers
		} else {
			parsedPeers, err := config.ParseBootstrapPeers(req.Arguments())
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			inputPeers = parsedPeers
		}

		if len(inputPeers) == 0 {
			res.SetError(errors.New("no bootstrap peers to add"), cmdkit.ErrClient)
			return
		}

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		added, err := bootstrapAdd(r, cfg, inputPeers)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{config.BootstrapPeerStrings(added)})
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*BootstrapOutput)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			if err := bootstrapWritePeers(buf, "added ", out.Peers); err != nil {
				return nil, err
			}

			return buf, nil
		},
	},
}

var bootstrapAddDefaultCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add default peers to the bootstrap list.",
		ShortDescription: `Outputs a list of peers that were added (that weren't already
in the bootstrap list).`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		defltPeers, err := config.DefaultBootstrapPeers()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		added, err := bootstrapAdd(r, cfg, defltPeers)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{config.BootstrapPeerStrings(added)})
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*BootstrapOutput)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			if err := bootstrapWritePeers(buf, "added ", out.Peers); err != nil {
				return nil, err
			}

			return buf, nil
		},
	},
}

var bootstrapRemoveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove peers from the bootstrap list.",
		ShortDescription: `Outputs the list of peers that were removed.
` + bootstrapSecurityWarning,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer", false, true, peerOptionDesc).EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("all", "Remove all bootstrap peers. (Deprecated, use 'all' subcommand)"),
	},
	Subcommands: map[string]*cmds.Command{
		"all": bootstrapRemoveAllCmd,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		all, _, err := req.Option("all").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var removed []config.BootstrapPeer
		if all {
			removed, err = bootstrapRemoveAll(r, cfg)
		} else {
			input, perr := config.ParseBootstrapPeers(req.Arguments())
			if perr != nil {
				res.SetError(perr, cmdkit.ErrNormal)
				return
			}

			removed, err = bootstrapRemove(r, cfg, input)
		}
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{config.BootstrapPeerStrings(removed)})
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*BootstrapOutput)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			err = bootstrapWritePeers(buf, "removed ", out.Peers)
			return buf, err
		},
	},
}

var bootstrapRemoveAllCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Remove all peers from the bootstrap list.",
		ShortDescription: `Outputs the list of peers that were removed.`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		removed, err := bootstrapRemoveAll(r, cfg)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&BootstrapOutput{config.BootstrapPeerStrings(removed)})
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*BootstrapOutput)
			if !ok {
				return nil, e.TypeErr(out, v)
			}

			buf := new(bytes.Buffer)
			err = bootstrapWritePeers(buf, "removed ", out.Peers)
			return buf, err
		},
	},
}

var bootstrapListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show peers in the bootstrap list.",
		ShortDescription: "Peers are output in the format '<multiaddr>/<peerID>'.",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		peers, err := cfg.BootstrapPeers()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		res.SetOutput(&BootstrapOutput{config.BootstrapPeerStrings(peers)})
	},
	Type: BootstrapOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: bootstrapMarshaler,
	},
}

func bootstrapMarshaler(res cmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	out, ok := v.(*BootstrapOutput)
	if !ok {
		return nil, e.TypeErr(out, v)
	}

	buf := new(bytes.Buffer)
	err = bootstrapWritePeers(buf, "", out.Peers)
	return buf, err
}

func bootstrapWritePeers(w io.Writer, prefix string, peers []string) error {

	sort.Stable(sort.StringSlice(peers))
	for _, peer := range peers {
		_, err := w.Write([]byte(prefix + peer + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}

func bootstrapAdd(r repo.Repo, cfg *config.Config, peers []config.BootstrapPeer) ([]config.BootstrapPeer, error) {
	addedMap := map[string]struct{}{}
	addedList := make([]config.BootstrapPeer, 0, len(peers))

	// re-add cfg bootstrap peers to rm dupes
	bpeers := cfg.Bootstrap
	cfg.Bootstrap = nil

	// add new peers
	for _, peer := range peers {
		s := peer.String()
		if _, found := addedMap[s]; found {
			continue
		}

		cfg.Bootstrap = append(cfg.Bootstrap, s)
		addedList = append(addedList, peer)
		addedMap[s] = struct{}{}
	}

	// add back original peers. in this order so that we output them.
	for _, s := range bpeers {
		if _, found := addedMap[s]; found {
			continue
		}

		cfg.Bootstrap = append(cfg.Bootstrap, s)
		addedMap[s] = struct{}{}
	}

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return addedList, nil
}

func bootstrapRemove(r repo.Repo, cfg *config.Config, toRemove []config.BootstrapPeer) ([]config.BootstrapPeer, error) {
	removed := make([]config.BootstrapPeer, 0, len(toRemove))
	keep := make([]config.BootstrapPeer, 0, len(cfg.Bootstrap))

	peers, err := cfg.BootstrapPeers()
	if err != nil {
		return nil, err
	}

	for _, peer := range peers {
		found := false
		for _, peer2 := range toRemove {
			if peer.Equal(peer2) {
				found = true
				removed = append(removed, peer)
				break
			}
		}

		if !found {
			keep = append(keep, peer)
		}
	}
	cfg.SetBootstrapPeers(keep)

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return removed, nil
}

func bootstrapRemoveAll(r repo.Repo, cfg *config.Config) ([]config.BootstrapPeer, error) {
	removed, err := cfg.BootstrapPeers()
	if err != nil {
		return nil, err
	}

	cfg.Bootstrap = nil
	if err := r.SetConfig(cfg); err != nil {
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
