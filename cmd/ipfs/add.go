package main

import (
	"fmt"
	"path/filepath"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

var cmdIpfsAdd = &commander.Command{
	UsageLine: "add",
	Short:     "Add an object to ipfs.",
	Long: `ipfs add <path>... - Add objects to ipfs.

    Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run:  addCmd,
	Flag: *flag.NewFlagSet("ipfs-add", flag.ExitOnError),
}

func init() {
	cmdIpfsAdd.Flag.Bool("r", false, "add objects recursively")
}

var addCmd = makeCommand(command{
	name:      "add",
	args:      1,
	flags:     []string{"r"},
	cmdFn:     commands.Add,
	argFilter: filepath.Abs,
})
