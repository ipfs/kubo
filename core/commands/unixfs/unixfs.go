package unixfs

import cmds "github.com/ipfs/go-ipfs/commands"

var UnixFSCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipfs objects representing Unix filesystems",
		ShortDescription: `
'ipfs unixfs' provides a familar interface to filesystems represtented
by IPFS objects that hides IPFS-implementation details like layout
objects (e.g. fanout and chunking).
`,
		Synopsis: `
ipfs unixfs ls <path>...  - List directory contents for <path>...
`,
	},

	Subcommands: map[string]*cmds.Command{
		"ls": LsCmd,
	},
}
