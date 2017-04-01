package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmSNbH2A1evCCbJSDC6u3RV3GGDhgu6pRGbXHvrN89tMKf/go-ipfs-cmdkit"
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
