package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsPin = &commander.Command{
	UsageLine: "pin",
	Short:     "",
	Long: `ipfs pin [add|rm] - object pinning commands
`,
	Subcommands: []*commander.Command{
		cmdIpfsSubPin,
		cmdIpfsSubUnpin,
	},
}

var cmdIpfsSubPin = &commander.Command{
	UsageLine: "add",
	Short:     "pin an ipfs object to local storage.",
	Long: `ipfs pin add <ipfs-path> - pin ipfs object to local storage.

    Retrieves the object named by <ipfs-path> and stores it locally
    on disk.
`,
	Run:  pinSubCmd,
	Flag: *flag.NewFlagSet("ipfs-pin", flag.ExitOnError),
}

var pinSubCmd = makeCommand(command{
	name:  "pin",
	args:  1,
	flags: []string{"r", "d"},
	cmdFn: commands.Pin,
})

var cmdIpfsSubUnpin = &commander.Command{
	UsageLine: "rm",
	Short:     "unpin an ipfs object from local storage.",
	Long: `ipfs pin rm <ipfs-path> - unpin ipfs object from local storage.

	Removes the pin from the given object allowing it to be garbage
	collected if needed.
`,
	Run:  unpinSubCmd,
	Flag: *flag.NewFlagSet("ipfs-unpin", flag.ExitOnError),
}

var unpinSubCmd = makeCommand(command{
	name:  "unpin",
	args:  1,
	flags: []string{"r", "d"},
	cmdFn: commands.Unpin,
})

func init() {
	cmdIpfsSubPin.Flag.Bool("r", false, "pin objects recursively")
	cmdIpfsSubPin.Flag.Int("d", 1, "recursive depth")
	cmdIpfsSubUnpin.Flag.Bool("r", false, "unpin objects recursively")
}
