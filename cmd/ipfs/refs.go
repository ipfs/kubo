package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	commands "github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsRefs = &commander.Command{
	UsageLine: "refs",
	Short:     "List link hashes from an object.",
	Long: `ipfs refs <ipfs-path> - List link hashes from an object..

    Retrieves the object named by <ipfs-path> and displays the link
    hashes it contains, with the following format:

    <link base58 hash>

    Note: list all refs recursively with -r.

`,
	Run:  refCmd,
	Flag: *flag.NewFlagSet("ipfs-refs", flag.ExitOnError),
}

func init() {
	cmdIpfsRefs.Flag.Bool("r", false, "recursive: list refs recursively")
	cmdIpfsRefs.Flag.Bool("u", false, "unique: list each ref only once")
}

var refCmd = makeCommand(command{
	name:  "refs",
	args:  1,
	flags: []string{"r", "u"},
	cmdFn: commands.Refs,
})
