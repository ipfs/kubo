package main

import (
	"fmt"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
	"os"
)

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

    config      Manage configuration.
    version     Show ipfs version information.
    commands    List all available commands.

Advanced Commands:

    mount       Mount a ipfs a read-only mountpoint.

Use "ipfs help <command>" for more information about a command.
`,
	Run: ipfsCmd,
	Subcommands: []*commander.Command{
		cmdIpfsVersion,
		// cmdIpfsConfig,
		cmdIpfsCommands,
	},
	Flag: *flag.NewFlagSet("ipfs", flag.ExitOnError),
}

func ipfsCmd(c *commander.Command, args []string) error {
	u.POut(c.Long)
	return nil
}

func main() {
	err := CmdIpfs.Dispatch(os.Args[1:])
	if err != nil {
		if len(err.Error()) > 0 {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		os.Exit(1)
	}
	return
}
