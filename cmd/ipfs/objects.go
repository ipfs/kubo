package main

import (
	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsObject = &commander.Command{
	UsageLine: "object",
	Short:     "interact with ipfs objects",
	Long: `ipfs object - interact with ipfs objects

		ipfs object data <key>  - return the data for this key as raw bytes
		ipfs object links <key> - lists (the keys of ?) the links this key points to
		ipfs object get <key>   - output dag object to stdout
		ipfs object put         - add dag object from stdin

ipfs object is a plumbing command used to manipulate dag objects directly.
- <key> is a base58 encoded multihash.
- It reads from stdin or writes to stdout.
- It accepts multiple encodings: --encoding=[ protobuf, json, ... ]`,
	Subcommands: []*commander.Command{
		cmdIpfsObjectData,
		cmdIpfsObjectLinks,
		cmdIpfsObjectGet,
		cmdIpfsObjectPut,
	},
	Flag: *flag.NewFlagSet("ipfs-object", flag.ExitOnError),
}

var cmdIpfsObjectData = &commander.Command{
	UsageLine: "data <key>",
	Short:     "data outputs the raw bytes named by <key>",
	Long: `ipfs data <key> - data outputs the raw bytes named by <key>

ipfs data is a plumbing command for retreiving the raw bytes stored in a dag node.
It outputs to stdout, and <key> is a base58 encoded multihash.`,
	Run: makeCommand(command{
		name:   "objectData",
		args:   1,
		flags:  nil,
		online: true,
		cmdFn:  commands.ObjectData,
	}),
}

var cmdIpfsObjectLinks = &commander.Command{
	UsageLine: "links <key>",
	Short:     "outputs the links pointed to by <key>",
	Long: `ipfs links <key> - outputs the links pointed to by <key>

ipfs block get is a plumbing command for retreiving raw ipfs blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.`,
	Run: makeCommand(command{
		name:   "objectLinks",
		args:   1,
		flags:  nil,
		online: true,
		cmdFn:  commands.ObjectLinks,
	}),
}

func init() {
	cmdIpfsObjectGet.Flag.String("encoding", "json", "the encoding to use..")
	cmdIpfsObjectPut.Flag.String("encoding", "json", "the encoding to use..")
}

var cmdIpfsObjectGet = &commander.Command{
	UsageLine: "get <key>",
	Short:     "get and serialize the dag node named by <key>",
	Long: `ipfs get <key> - get and output the dag node named by <key>

ipfs object get is a plumbing command for retreiving dag nodes.
It serialize the dag node to the format specified by the format flag.
It outputs to stdout, and <key> is a base58 encoded multihash.

Formats:

This command outputs and accepts data in a variety of encodings: protobuf, json, etc.
Use the --encoding flag
`,
	Run: makeCommand(command{
		name:   "blockGet",
		args:   1,
		flags:  []string{"encoding"},
		online: true,
		cmdFn:  commands.ObjectGet,
	}),
}

var cmdIpfsObjectPut = &commander.Command{
	UsageLine: "put",
	Short:     "store stdin as a dag object, outputs <key>",
	Long: `ipfs put - store stdin as a dag object, outputs <key>

ipfs object put is a plumbing command for storing dag nodes.
It serialize the dag node to the format specified by the format flag.
It reads from stding, and <key> is a base58 encoded multihash.

Formats:

This command outputs and accepts data in a variety of encodings: protobuf, json, etc.
Use the --encoding flag`,
	Run: makeCommand(command{
		name:   "blockPut",
		args:   0,
		flags:  []string{"encoding"},
		online: true,
		cmdFn:  commands.ObjectPut,
	}),
}
