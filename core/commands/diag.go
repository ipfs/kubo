package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmNaA1HxkbVtweGfabDMy2DMLvqQ1eg3LNEqDMVA3zCoz1/go-ipfs-cmdkit"
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
