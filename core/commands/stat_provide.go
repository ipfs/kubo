package commands

import (
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
)

var statProvideCmd = &cmds.Command{
	Status: cmds.Deprecated,
	Helptext: cmds.HelpText{
		Tagline: "Deprecated command, use 'ipfs stats reprovide' instead.",
		ShortDescription: `
'ipfs stats provide' is deprecated because provide and reprovide operations
are now distinct. This command may be replaced by provide only stats in the
future.
`,
	},
	Arguments: []cmds.Argument{},
	Options:   []cmds.Option{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		stats, err := nd.Provider.Stat()
		if err != nil {
			return err
		}
		_, fullRT := nd.DHTClient.(*fullrt.FullRT)

		if err := res.Emit(reprovideStats{stats, fullRT}); err != nil {
			return err
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s reprovideStats) error {
			wtr := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			defer wtr.Flush()

			fmt.Fprintf(wtr, "TotalProvides:\t%s\n", humanNumber(s.TotalReprovides))
			fmt.Fprintf(wtr, "AvgProvideDuration:\t%s\n", humanDuration(s.AvgReprovideDuration))
			fmt.Fprintf(wtr, "LastReprovideDuration:\t%s\n", humanDuration(s.LastReprovideDuration))
			if !s.LastRun.IsZero() {
				fmt.Fprintf(wtr, "LastRun:\t%s\n", humanTime(s.LastRun))
				if s.fullRT {
					fmt.Fprintf(wtr, "NextRun:\t%s\n", humanTime(s.LastRun.Add(s.ReprovideInterval)))
				}
			}
			return nil
		}),
	},
	Type: reprovideStats{},
}
