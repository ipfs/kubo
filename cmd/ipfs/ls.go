package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsLs = &commander.Command{
	UsageLine: "ls",
	Short:     "List links from an object.",
	Long: `ipfs ls <ipfs-path> - List links from an object.

    Retrieves the object named by <ipfs-path> and displays the links
    it contains, with the following format:

    <link base58 hash> <link size in bytes> <link name>

`,
	Run:  lsCmd,
	Flag: *flag.NewFlagSet("ipfs-ls", flag.ExitOnError),
}

var lsCmd = MakeCommand("ls", nil, commands.Ls)
