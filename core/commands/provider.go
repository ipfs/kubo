package commands

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"io"
)

// ProviderCmd provides command line access to the provider system
var ProviderCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Interact with the provider subsystem",
		ShortDescription: "",
	},

	Subcommands: map[string]*cmds.Command{
		"tracking":  trackingProviderCmd,
		"reprovide": reprovideCmd,
	},
}

var trackingProviderCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List the cids being tracked by the provider system.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return nil
		}

		cids, err := node.Provider.Tracking()
		if err != nil {
			res.CloseWithError(err)
			return err
		}

		for c := range cids {
			res.Emit(c)
		}

		res.Close()
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			fmt.Fprintf(w, "%s\n", c.String())
			return nil
		}),
	},
	Type: cid.Cid{},
}

var reprovideCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce tracked cids to the network.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		err = nd.Provider.Reprovide(req.Context)
		if err != nil {
			return err
		}

		return nil
	},
}
