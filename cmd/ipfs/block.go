package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsBlock = &commander.Command{
	UsageLine: "block",
	Short:     "get/put **raw** ipfs blocks",
	Long: `ipfs block (get|put) - get/put **raw** ipfs blocks.

    ipfs block get <key> > valfile    - get block of <key> and write it to valfile
    ipfs block put <key>  < valfile   - pipe valfile to block <key>
Examples:

  Get the value of the 'datastore.path' key:

      ipfs config datastore.path

  Set the value of the 'datastore.path' key:

      ipfs config datastore.path ~/.go-ipfs/datastore

`,
	// Run: blockGetCmd,
	Subcommands: []*commander.Command{
		cmdIpfsBlockGet,
		cmdIpfsBlockPut,
	},
	Flag: *flag.NewFlagSet("ipfs-block", flag.ExitOnError),
}

var cmdIpfsBlockGet = &commander.Command{
	UsageLine: "get",
	Short:     "get a row ipfs block",
	Run: makeCommand(command{
		name:   "get",
		args:   1,
		flags:  nil,
		online: true,
		cmdFn:  commands.BlockGet,
	}),
}

var cmdIpfsBlockPut = &commander.Command{
	UsageLine: "put",
	Short:     "put a row ipfs block",
	Run: makeCommand(command{
		name:   "put",
		args:   1,
		flags:  nil,
		online: true,
		cmdFn:  commands.BlockPut,
	}),
}
