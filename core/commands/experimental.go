package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"
)

var ExpCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Experimental comands",
		ShortDescription: `'ipfs exp' is grouping experimental features that may be changed or removed at any time.`,
	},

	Subcommands: map[string]*cmds.Command{
		"corenet": CorenetCmd,
	},
}
