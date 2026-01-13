package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var statProvideCmd = &cmds.Command{
	Status: cmds.Deprecated,
	Helptext: cmds.HelpText{
		Tagline: "Deprecated command, use 'ipfs provide stat' instead.",
		ShortDescription: `
'ipfs stats provide' is deprecated because provide and reprovide operations
are now distinct. This command may be replaced by provide only stats in the
future.
`,
	},
	Arguments: provideStatCmd.Arguments,
	Options:   provideStatCmd.Options,
	Run:       provideStatCmd.Run,
	Encoders:  provideStatCmd.Encoders,
	Type:      provideStatCmd.Type,
}
