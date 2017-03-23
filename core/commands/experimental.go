package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"
)

var ExpCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Experimental commands",
		ShortDescription: `'ipfs exp' groups experimental features that are subject to change or removal at any time.`,
	},
	Subcommands: map[string]*cmds.Command{
		"corenet": CorenetCmd,
	},
}
