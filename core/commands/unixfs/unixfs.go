package unixfs

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var UnixFSCmd = &cmds.Command{
	Status: cmds.Deprecated, // https://github.com/ipfs/kubo/pull/7755
	Helptext: cmds.HelpText{
		Tagline: "Interact with IPFS objects representing Unix filesystems.",
		ShortDescription: `
Old interface to file systems represented by UnixFS.
Superseded by modern alternatives: 'ipfs ls' and 'ipfs files'
`,
	},

	Subcommands: map[string]*cmds.Command{
		"ls": LsCmd,
	},
}
