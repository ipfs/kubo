package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"

	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	daemon "github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

// The IPFS command tree. It is an instance of `commander.Command`.
var CmdIpfs = &commander.Command{
	UsageLine: "ipfs [<flags>] <command> [<args>]",
	Short:     "global versioned p2p merkledag file system",
	Long: `ipfs - global versioned p2p merkledag file system

Basic commands:

    init          Initialize ipfs local configuration.
    add <path>    Add an object to ipfs.
    cat <ref>     Show ipfs object data.
    ls <ref>      List links from an object.
    refs <ref>    List link hashes from an object.

Tool commands:

    config        Manage configuration.
    version       Show ipfs version information.
    commands      List all available commands.

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint.
    serve         Serve an interface to ipfs.

Use "ipfs help <command>" for more information about a command.
`,
	Run: ipfsCmd,
	Subcommands: []*commander.Command{
		cmdIpfsAdd,
		cmdIpfsCat,
		cmdIpfsLs,
		cmdIpfsRefs,
		cmdIpfsConfig,
		cmdIpfsVersion,
		cmdIpfsCommands,
		cmdIpfsMount,
		cmdIpfsInit,
		cmdIpfsServe,
		cmdIpfsRun,
		cmdIpfsName,
		cmdIpfsBootstrap,
	},
	Flag: *flag.NewFlagSet("ipfs", flag.ExitOnError),
}

// log is the command logger
var log = u.Logger("cmd/ipfs")

func init() {
	config, err := config.PathRoot()
	if err != nil {
		u.POut("Failure initializing the default Config Directory: ", err)
		os.Exit(1)
	}
	CmdIpfs.Flag.String("c", config, "specify config directory")
}

func ipfsCmd(c *commander.Command, args []string) error {
	u.POut(c.Long)
	return nil
}

func main() {
	u.Debug = false

	// setup logging
	// u.SetupLogging() done in an init() block now.

	// if debugging, setup profiling.
	if u.Debug {
		ofi, err := os.Create("cpu.prof")
		if err != nil {
			fmt.Println(err)
			return
		}
		pprof.StartCPUProfile(ofi)
		defer ofi.Close()
		defer pprof.StopCPUProfile()
	}

	err := CmdIpfs.Dispatch(os.Args[1:])
	if err != nil {
		if len(err.Error()) > 0 {
			fmt.Fprintf(os.Stderr, "ipfs %s: %v\n", os.Args[1], err)
		}
		os.Exit(1)
	}
	return
}

// localNode constructs a node
func localNode(confdir string, online bool) (*core.IpfsNode, error) {
	filename, err := config.Filename(confdir)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(filename)
	if err != nil {
		return nil, err
	}

	if cfg.Version.Check != "ignore" {
		obsolete := checkForUpdates()
		if obsolete != nil {
			fmt.Println(obsolete)
			if cfg.Version.Check == "error" {
				return nil, obsolete
			}
		}
	}

	return core.NewIpfsNode(cfg, online)
}

// Gets the config "-c" flag from the command, or returns
// the default configuration root directory
func getConfigDir(c *commander.Command) (string, error) {

	// use the root cmd (that's where config is specified)
	for ; c.Parent != nil; c = c.Parent {
	}

	// flag should be defined on root.
	param := c.Flag.Lookup("c").Value.Get().(string)
	if param != "" {
		return u.TildeExpansion(param)
	}

	return config.PathRoot()
}

func getConfig(c *commander.Command) (*config.Config, error) {
	confdir, err := getConfigDir(c)
	if err != nil {
		return nil, err
	}

	filename, err := config.Filename(confdir)
	if err != nil {
		return nil, err
	}

	return config.Load(filename)
}

// cmdContext is a wrapper structure that keeps a node, a daemonlistener, and
// a config directory together. These three are needed for most commands.
type cmdContext struct {
	node      *core.IpfsNode
	daemon    *daemon.DaemonListener
	configDir string
}

// setupCmdContext initializes a cmdContext structure from a given command.
func setupCmdContext(c *commander.Command, online bool) (cc cmdContext, err error) {
	rootCmd := c
	for ; rootCmd.Parent != nil; rootCmd = c.Parent {
	}

	cc.configDir, err = getConfigDir(rootCmd)
	if err != nil {
		return
	}

	cc.node, err = localNode(cc.configDir, online)
	if err != nil {
		return
	}

	cc.daemon, err = setupDaemon(cc.configDir, cc.node)
	if err != nil {
		return
	}

	return
}

// setupDaemon sets up the daemon corresponding to given node.
func setupDaemon(confdir string, node *core.IpfsNode) (*daemon.DaemonListener, error) {
	if node.Config.Addresses.API == "" {
		return nil, errors.New("no config.Addresses.API endpoint supplied")
	}

	maddr, err := ma.NewMultiaddr(node.Config.Addresses.API)
	if err != nil {
		return nil, err
	}

	dl, err := daemon.NewDaemonListener(node, maddr, confdir)
	if err != nil {
		return nil, err
	}
	go dl.Listen()
	return dl, nil
}
