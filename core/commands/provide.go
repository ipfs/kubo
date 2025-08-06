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
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Control providing operations",
		ShortDescription: `
Control providing operations.

NOTE: This command is experimental and not all provide-related commands have
been migrated to this namespace yet. For example, 'ipfs routing
provide|reprovide' are still under the routing namespace, 'ipfs stats
reprovide' provides statistics. Additionally, 'ipfs bitswap reprovide' and
'ipfs stats provide' are deprecated.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"clear": ProvideClearCmd,
	},
}

var ProvideClearCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Clear all CIDs from the provide queue.",
		ShortDescription: `
Clear all CIDs from the reprovide queue.

Note: Kubo will automatically clear the queue when it detects a change of
Reprovider.Strategy upon a restart. For more information about reprovider
strategies, see:
https://github.com/ipfs/kubo/blob/master/docs/config.md#reproviderstrategy
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
