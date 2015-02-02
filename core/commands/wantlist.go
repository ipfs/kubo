package commands

import cmds "github.com/jbenet/go-ipfs/commands"

var WantlistCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "A set of commands to work with the bitswap wantlist",
		ShortDescription: ``,
	},
	Subcommands: map[string]*cmds.Command{
		"show": showWantlistCmd,
	},
}

var showWantlistCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show blocks currently on the wantlist",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer`,
	},
	Type: KeyList{},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyList{nd.Exchange.GetWantlist()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: KeyListTextMarshaler,
	},
}
