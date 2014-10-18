package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsUpdate = &commander.Command{
	UsageLine: "update",
	Short:     "check for updates and apply them",
	Long: `ipfs update - check for updates and apply them

		ipfs update <version>  - apply
		ipfs update check      - just check
		ipfs update log        - list the changelogs

ipfs update is a utility command used to check for updates and apply them.
I wont even try, @jbenet. You do this much better :)`,
	Run: makeCommand(command{
		name:   "updateApply",
		args:   0,
		flags:  nil,
		online: true,
		cmdFn:  commands.UpdateApply,
	}),
	Subcommands: []*commander.Command{
		cmdIpfsUpdateCheck,
		cmdIpfsUpdateLog,
	},
	Flag: *flag.NewFlagSet("ipfs-update", flag.ExitOnError),
}

var cmdIpfsUpdateCheck = &commander.Command{
	UsageLine: "check",
	Short:     "",
	Long:      `ipfs update check <key>`,
	Run: makeCommand(command{
		name:   "updateCheck",
		args:   0,
		flags:  nil,
		online: false,
		cmdFn:  commands.UpdateCheck,
	}),
}

var cmdIpfsUpdateLog = &commander.Command{
	UsageLine: "log",
	Short:     "list the last versions and their changelog",
	Long:      `ipfs updage log - list the last versions and their changelog`,
	Run: makeCommand(command{
		name:   "updateLog",
		args:   0,
		flags:  nil,
		online: false,
		cmdFn:  commands.UpdateCheck,
	}),
}
