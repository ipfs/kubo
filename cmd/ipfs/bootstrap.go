package main

import (
	"errors"
	"strings"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	config "github.com/jbenet/go-ipfs/config"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsBootstrap = &commander.Command{
	UsageLine: "bootstrap",
	Short:     "Show a list of bootstrapped addresses.",
	Long: `ipfs bootstrap - show, or manipulate bootstrap node addresses

Running 'ipfs bootstrap' with no arguments will run 'ipfs bootstrap list'.

Commands:

	list               Show the boostrap list.
	add <address>      Add a node's address to the bootstrap list.
	remove <address>   Remove an address from the bootstrap list.

` + bootstrapSecurityWarning,
	Run: bootstrapListCmd,
	Subcommands: []*commander.Command{
		cmdIpfsBootstrapRemove,
		cmdIpfsBootstrapAdd,
		cmdIpfsBootstrapList,
	},
	Flag: *flag.NewFlagSet("ipfs-bootstrap", flag.ExitOnError),
}

var cmdIpfsBootstrapRemove = &commander.Command{
	UsageLine: "remove <address | peerid>",
	Short:     "Remove addresses from the bootstrap list.",
	Long: `ipfs bootstrap remove - remove addresses from the bootstrap list
` + bootstrapSecurityWarning,
	Run:  bootstrapRemoveCmd,
	Flag: *flag.NewFlagSet("ipfs-bootstrap-remove", flag.ExitOnError),
}

var cmdIpfsBootstrapAdd = &commander.Command{
	UsageLine: "add <address | peerid>",
	Short:     "Add addresses to the bootstrap list.",
	Long: `ipfs bootstrap add - add addresses to the bootstrap list
` + bootstrapSecurityWarning,
	Run:  bootstrapAddCmd,
	Flag: *flag.NewFlagSet("ipfs-bootstrap-add", flag.ExitOnError),
}

var cmdIpfsBootstrapList = &commander.Command{
	UsageLine: "list",
	Short:     "Show addresses in the bootstrap list.",
	Run:       bootstrapListCmd,
	Flag:      *flag.NewFlagSet("ipfs-bootstrap-list", flag.ExitOnError),
}

func bootstrapRemoveCmd(c *commander.Command, inp []string) error {

	if len(inp) == 0 {
		return errors.New("remove: no address or peerid specified")
	}

	toRemove, err := bootstrapInputToPeers(inp)
	if err != nil {
		return err
	}

	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	keep := []*config.BootstrapPeer{}
	remove := []*config.BootstrapPeer{}

	// function to filer what to keep
	shouldKeep := func(bp *config.BootstrapPeer) bool {
		for _, skipBP := range toRemove {

			// IDs must match to skip.
			if bp.PeerID != skipBP.PeerID {
				continue
			}

			// if Addresses match, or skipBP has no addr (wildcard)
			if skipBP.Address == bp.Address || skipBP.Address == "" {
				return false
			}
		}
		return true
	}

	// filter all the existing peers
	for _, currBP := range cfg.Bootstrap {
		if shouldKeep(currBP) {
			keep = append(keep, currBP)
		} else {
			remove = append(remove, currBP)
		}
	}

	// if didn't remove anyone, bail.
	if len(keep) == len(cfg.Bootstrap) {
		return errors.New("remove: peer given did not match any in list")
	}

	// write new config
	cfg.Bootstrap = keep
	if err := writeConfig(c, cfg); err != nil {
		return err
	}

	for _, bp := range remove {
		u.POut("removed %s\n", bp)
	}
	return nil
}

func bootstrapAddCmd(c *commander.Command, inp []string) error {

	if len(inp) == 0 {
		return errors.New("add: no address specified")
	}

	toAdd, err := bootstrapInputToPeers(inp)
	if err != nil {
		return err
	}

	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	// function to check whether a peer is already in the list.
	combine := func(lists ...[]*config.BootstrapPeer) []*config.BootstrapPeer {

		set := map[string]struct{}{}
		final := []*config.BootstrapPeer{}

		for _, list := range lists {
			for _, peer := range list {
				// if already in the set, continue
				_, found := set[peer.String()]
				if found {
					continue
				}

				set[peer.String()] = struct{}{}
				final = append(final, peer)
			}
		}
		return final
	}

	// combine both lists, removing dups.
	cfg.Bootstrap = combine(cfg.Bootstrap, toAdd)
	if err := writeConfig(c, cfg); err != nil {
		return err
	}

	for _, bp := range toAdd {
		u.POut("added %s\n", bp)
	}
	return nil
}

func bootstrapListCmd(c *commander.Command, inp []string) error {

	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	for _, bp := range cfg.Bootstrap {
		u.POut("%s\n", bp)
	}

	return nil
}

func writeConfig(c *commander.Command, cfg *config.Config) error {

	confdir, err := getConfigDir(c)
	if err != nil {
		return err
	}

	filename, err := config.Filename(confdir)
	if err != nil {
		return err
	}

	return config.WriteConfigFile(filename, cfg)
}

func bootstrapInputToPeers(input []string) ([]*config.BootstrapPeer, error) {
	split := func(addr string) (string, string) {
		idx := strings.LastIndex(addr, "/")
		if idx == -1 {
			return "", addr
		}
		return addr[:idx], addr[idx+1:]
	}

	peers := []*config.BootstrapPeer{}
	for _, addr := range input {
		addrS, peeridS := split(addr)

		// make sure addrS parses as a multiaddr.
		if len(addrS) > 0 {
			maddr, err := ma.NewMultiaddr(addrS)
			if err != nil {
				return nil, err
			}

			addrS, err = maddr.String()
			if err != nil {
				return nil, err
			}
		}

		// make sure idS parses as a peer.ID
		peerid, err := mh.FromB58String(peeridS)
		if err != nil {
			return nil, err
		}

		// construct config entry
		peers = append(peers, &config.BootstrapPeer{
			Address: addrS,
			PeerID:  peer.ID(peerid).Pretty(),
		})
	}
	return peers, nil
}

const bootstrapSecurityWarning = `
SECURITY WARNING:

The bootstrap command manipulates the "bootstrap list", which contains
the addresses of bootstrap nodes. These are the *trusted peers* from
which to learn about other peers in the network. Only edit this list
if you understand the risks of adding or removing nodes from this list.

`
