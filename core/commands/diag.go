package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmVViZcg6N29WMrbfbzuYXFAGVoCvcR5oqadxfnMcLMnmx/go-ipfs-cmdkit"
)

var DiagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Generate diagnostic reports.",
	},

	Subcommands: map[string]*cmds.Command{
		"sys":  sysDiagCmd,
		"cmds": ActiveReqsCmd,
	},
}
