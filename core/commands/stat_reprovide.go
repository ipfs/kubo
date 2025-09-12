package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var statReprovideCmd = &cmds.Command{
	Status: cmds.Deprecated,
	Helptext: cmds.HelpText{
		Tagline: "Deprecated command, use 'ipfs provide stat' instead.",
		ShortDescription: `
'ipfs stats reprovide' is deprecated because provider stats are now
available from 'ipfs provide stat'.
`,
	},
	Arguments: provideStatCmd.Arguments,
	Options:   provideStatCmd.Options,
	Run:       provideStatCmd.Run,
	Encoders:  provideStatCmd.Encoders,
	Type:      provideStatCmd.Type,
}
