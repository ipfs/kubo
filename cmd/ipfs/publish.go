package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsPub = &commander.Command{
	UsageLine: "publish",
	Short:     "Publish an object to ipns under your key.",
	Long: `ipfs publish <path> - Publish object to ipns.

`,
	Run:  pubCmd,
	Flag: *flag.NewFlagSet("ipfs-publish", flag.ExitOnError),
}

func init() {
	cmdIpfsPub.Flag.String("k", "", "Specify key to use for publishing.")
}

var pubCmd = makeCommand(command{
	name:   "publish",
	args:   1,
	flags:  []string{"k"},
	online: true,
	cmdFn:  commands.Publish,
})
