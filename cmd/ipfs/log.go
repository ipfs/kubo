package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsLog = &commander.Command{
	UsageLine: "log <name> <level> ",
	Short:     "switch logging levels of a running daemon",
	Long: `ipfs log <name> <level> - switch logging levels of a running daemon

   <name> is a the subsystem logging identifier. Use * for all subsystems.
   <level> is one of: debug, info, notice, warning, error, critical

ipfs log is a utility command used to change the logging output of a running daemon.
`,
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
