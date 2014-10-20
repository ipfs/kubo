package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsPin = &commander.Command{
	UsageLine: "pin",
	Short:     "pin an ipfs object to local storage.",
	Long: `ipfs pin <ipfs-path> - pin ipfs object to local storage.

    Retrieves the object named by <ipfs-path> and stores it locally
    on disk.
`,
	Run:  pinCmd,
	Flag: *flag.NewFlagSet("ipfs-pin", flag.ExitOnError),
}

var pinCmd = makeCommand(command{
	name:  "pin",
	args:  1,
	flags: []string{"r", "d"},
	cmdFn: commands.Pin,
})

var cmdIpfsUnpin = &commander.Command{
	UsageLine: "unpin",
	Short:     "unpin an ipfs object from local storage.",
	Long: `ipfs unpin <ipfs-path> - unpin ipfs object from local storage.

	Removes the pin from the given object allowing it to be garbage
	collected if needed.
`,
	Run:  unpinCmd,
	Flag: *flag.NewFlagSet("ipfs-unpin", flag.ExitOnError),
}

var unpinCmd = makeCommand(command{
	name:  "unpin",
	args:  1,
	flags: []string{"r", "d"},
	cmdFn: commands.Unpin,
})

func init() {
	cmdIpfsPin.Flag.Bool("r", false, "pin objects recursively")
	cmdIpfsPin.Flag.Int("d", 1, "recursive depth")
	cmdIpfsUnpin.Flag.Bool("r", false, "unpin objects recursively")
}
