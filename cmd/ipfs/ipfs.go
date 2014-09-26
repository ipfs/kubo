package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

// The IPFS command tree. It is an instance of `commander.Command`.
var CmdIpfs = &commander.Command{
	UsageLine: "ipfs [<flags>] <command> [<args>]",
	Short:     "global versioned p2p merkledag file system",
	Long: `ipfs - global versioned p2p merkledag file system

Basic commands:

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
    webserver     Start a webserver

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
		cmdIpfsWebserver,
		cmdIpfsInit,
	},
	Flag: *flag.NewFlagSet("ipfs", flag.ExitOnError),
}

func init() {
	CmdIpfs.Flag.String("c", config.DefaultPathRoot, "specify config directory")
}

func ipfsCmd(c *commander.Command, args []string) error {
	u.POut(c.Long)
	return nil
}

func main() {
	u.Debug = true
	err := CmdIpfs.Dispatch(os.Args[1:])
	if err != nil {
		if len(err.Error()) > 0 {
			fmt.Fprintf(os.Stderr, "ipfs %s: %v\n", os.Args[1], err)
		}
		os.Exit(1)
	}
	return
}

func localNode(confdir string, online bool) (*core.IpfsNode, error) {
	cfg, err := config.Load(confdir + "/config")
	if err != nil {
		return nil, err
	}

	return core.NewIpfsNode(cfg, online)
}

// Gets the config "-c" flag from the command, or returns
// the empty string
func getConfigDir(c *commander.Command) (string, error) {
	conf := c.Flag.Lookup("c").Value.Get()
	if conf == nil {
		return "", nil
	}
	confStr, ok := conf.(string)
	if !ok {
		return "", errors.New("failed to retrieve config flag value.")
	}
	return confStr, nil
}
