package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsResolve = &commander.Command{
	UsageLine: "resolve",
	Short:     "resolve an ipns link to a hash",
	Long: `ipfs resolve <hash>... - Resolve hash.

`,
	Run:  resolveCmd,
	Flag: *flag.NewFlagSet("ipfs-resolve", flag.ExitOnError),
}

var resolveCmd = makeCommand(command{
	name:   "resolve",
	args:   1,
	flags:  nil,
	online: true,
	cmdFn:  commands.Resolve,
})
