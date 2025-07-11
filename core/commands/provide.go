package commands

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
)

const (
	provideQuietOptionName = "quiet"
)

var ProvideCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Control provide opertaions.",
		ShortDescription: `
'ipfs provide' is a utility command used to control provide operations.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"clear": provideClearCmd,
	},
}

var provideClearCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Clear all CIDs from the provide queue.",
		ShortDescription: `
'ipfs provide clear' is a utility command that removes all CIDs
from the current provide queue, both in memory and the datastore.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(provideQuietOptionName, "q", "Do not write output."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		quiet, _ := req.Options[provideQuietOptionName].(bool)
		if n.Provider == nil {
			return nil
		}

		cleared := n.Provider.Clear()
		if quiet {
			return nil
		}
		_ = re.Emit(cleared)

		return nil
	},
	Type: int(0),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, cleared int) error {
			quiet, _ := req.Options[provideQuietOptionName].(bool)
			if quiet {
				return nil
			}

			_, err := fmt.Fprintf(w, "removed %d items from provide queue\n", cleared)
			return err
		}),
	},
}
