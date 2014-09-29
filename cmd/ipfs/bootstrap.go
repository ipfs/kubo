package main

import (
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	config "github.com/jbenet/go-ipfs/config"
	"strings"
)

var cmdIpfsBootstrap = &commander.Command{
	UsageLine: "bootstrap",
	Short:     "Show a list of bootstrapped addresses.",
	Long: `ipfs bootstrap - show, or manipulate bootstrap node addresses

SECURITY WARNING:

The bootstrap command manipulates the "bootstrap list", which contains
the addresses of bootstrap nodes. These are the *trusted peers* from
which to learn about other peers in the network. Only edit this list
if you understand the risks of adding or removing nodes from this list.

Running 'ipfs bootstrap' with no arguments will run 'ipfs bootstrap list'.

Commands:

  list               Show the boostrap list.
  add <address>      Add a node's address to the bootstrap list.
  remove <address>   Remove an address from the bootstrap list.

`,
	Run: bootstrapCmd,
	Subcommands: []*commander.Command{
		cmdIpfsBootstrapRemove,
		cmdIpfsBootstrapAdd,
	},
	Flag: *flag.NewFlagSet("ipfs-bootstrap", flag.ExitOnError),
}

var cmdIpfsBootstrapRemove = &commander.Command{
	UsageLine: "remove",
	Run:       IpfsBootstrapRemoveCmd,
	Flag:      *flag.NewFlagSet("ipfs-bootstrap-remove", flag.ExitOnError),
}

var cmdIpfsBootstrapAdd = &commander.Command{
	UsageLine: "add",
	Run:       IpfsBootstrapAddCmd,
	Flag:      *flag.NewFlagSet("ipfs-bootstrap-add", flag.ExitOnError),
}

func IpfsBootstrapRemoveCmd(c *commander.Command, inp []string) error {

	if len(inp) == 0 {
		fmt.Println("No peer specified.")
		return nil
	}

	if strings.Contains(inp[0], "/") {

		var pID = inp[0][len(inp[0])-46:]
		var ip = strings.TrimSuffix(inp[0], pID)
		maddr, err := ma.NewMultiaddr(strings.TrimSuffix(ip, "/"))
		var address, _ = maddr.String()
		if err != nil {
			return err
		}

		peer := config.BootstrapPeer{
			Address: address,
			PeerID:  pID,
		}

		configpath, _ := config.Filename("~/.go-ipfs/config")
		var cfg config.Config
		readErr := config.ReadConfigFile(configpath, &cfg)
		if readErr != nil {
			return readErr
		}

		i := 0
		for _, v := range cfg.Bootstrap {
			if v.PeerID == peer.PeerID && v.Address == peer.Address {
				continue
			}
			cfg.Bootstrap[i] = v
			i++
		}
		cfg.Bootstrap = cfg.Bootstrap[:i]

		writeErr := config.WriteConfigFile(configpath, cfg)
		if writeErr != nil {
			return writeErr
		}
	}

	if !strings.Contains(inp[0], "/") {

		var peerID = inp[0]

		configpath, _ := config.Filename("~/.go-ipfs/config")
		var cfg config.Config
		readErr := config.ReadConfigFile(configpath, &cfg)
		if readErr != nil {
			return readErr
		}

		i := 0
		for _, v := range cfg.Bootstrap {
			if v.PeerID == peerID {
				continue
			}
			cfg.Bootstrap[i] = v
			i++
		}
		cfg.Bootstrap = cfg.Bootstrap[:i]

		writeErr := config.WriteConfigFile(configpath, cfg)
		if writeErr != nil {
			return writeErr
		}

	}
	return nil
}

func IpfsBootstrapAddCmd(c *commander.Command, inp []string) error {

	if len(inp) == 0 {
		fmt.Println("No peer specified.")
		return nil
	}

	var pID = inp[0][len(inp[0])-46:]
	var ip = strings.TrimSuffix(inp[0], pID)
	maddr, err := ma.NewMultiaddr(strings.TrimSuffix(ip, "/"))
	var address, _ = maddr.String()
	if err != nil {
		return err
	}

	peer := config.BootstrapPeer{
		Address: address,
		PeerID:  pID,
	}

	configpath, _ := config.Filename("~/.go-ipfs/config")
	var cfg config.Config
	readErr := config.ReadConfigFile(configpath, &cfg)
	if readErr != nil {
		return readErr
	}

	addedPeer := append(cfg.Bootstrap, &peer)
	cfg.Bootstrap = addedPeer

	writeErr := config.WriteConfigFile(configpath, cfg)
	if writeErr != nil {
		return writeErr
	}
	return nil
	return nil
}
func bootstrapCmd(c *commander.Command, inp []string) error {

	configpath, _ := config.Filename("~/.go-ipfs/config")
	var cfg config.Config
	config.ReadConfigFile(configpath, &cfg)

	for i := range cfg.Bootstrap {
		s := []string{cfg.Bootstrap[i].Address, "/", cfg.Bootstrap[i].PeerID, "\n"}
		fmt.Printf(strings.Join(s, ""))
	}

	return nil

}
