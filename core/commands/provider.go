package commands

import (
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "gx/ipfs/QmQtQrtNioesAWtrx8csBvfY37gTe94d6wQ3VikZUjxD39/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var ProviderCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Interact with the provider module.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"reprovide": reprovideCmd,
	},
}

var reprovideCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce our data to network.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.OnlineMode() {
			return ErrNotOnline
		}

		err = nd.Reprovider.Trigger(req.Context)
		if err != nil {
			return err
		}

		return nil
	},
}
