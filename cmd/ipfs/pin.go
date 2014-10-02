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

func init() {
	cmdIpfsPin.Flag.Bool("r", false, "pin objects recursively")
	cmdIpfsPin.Flag.Int("d", 1, "recursive depth")
}

var pinCmd = makeCommand(command{
	name:  "pin",
	args:  1,
	flags: []string{"r", "d"},
	cmdFn: commands.Pin,
})
