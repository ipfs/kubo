package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsLog = &commander.Command{
	UsageLine: "log",
	Short:     "switch logging levels of the daemon",
	Long: `ipfs log - manipulate raw ipfs blocks

    ipfs log dht error       - log error messages from the dht subsystem
    ipfs log merkledag debug - print debug output from the merkledag subsystem
		ipfs log * critical      - change all subsystems to only log critical errors

ipfs block is a utility command used to change the logging output of a running daemon.`,
	Run:  logCmd,
	Flag: *flag.NewFlagSet("ipfs-log", flag.ExitOnError),
}

var logCmd = makeCommand(command{
	name:   "log",
	args:   2,
	flags:  nil,
	online: true,
	cmdFn:  commands.Log,
})
