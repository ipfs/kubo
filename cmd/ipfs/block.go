package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsBlock = &commander.Command{
	UsageLine: "block",
	Short:     "manipulate raw ipfs blocks",
	Long: `ipfs block - manipulate raw ipfs blocks

    ipfs block get <key>  - get and output block named by <key>
    ipfs block put        - store stdin as a block, outputs <key>

ipfs block is a plumbing command used to manipulate raw ipfs blocks.
Reads from stdin or writes to stdout, and <key> is a base58 encoded
multihash.`,
	// Run: blockGetCmd,
	Subcommands: []*commander.Command{
		cmdIpfsBlockGet,
		cmdIpfsBlockPut,
	},
	Flag: *flag.NewFlagSet("ipfs-block", flag.ExitOnError),
}

var cmdIpfsBlockGet = &commander.Command{
	UsageLine: "get <key>",
	Short:     "get and output block named by <key>",
	Long: `ipfs get <key> - get and output block named by <key>

ipfs block get is a plumbing command for retreiving raw ipfs blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.`,
	Run: makeCommand(command{
		name:   "blockGet",
		args:   1,
		flags:  nil,
		online: true,
		cmdFn:  commands.BlockGet,
	}),
}

var cmdIpfsBlockPut = &commander.Command{
	UsageLine: "put",
	Short:     "store stdin as a block, outputs <key>",
	Long: `ipfs put - store stdin as a block, outputs <key>

ipfs block put is a plumbing command for storing raw ipfs blocks.
It reads from stding, and <key> is a base58 encoded multihash.`,
	Run: makeCommand(command{
		name:   "blockPut",
		args:   0,
		flags:  nil,
		online: true,
		cmdFn:  commands.BlockPut,
	}),
}
