package commands

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/ipfs/boxo/provider"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"golang.org/x/exp/constraints"
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
		"clear": provideClearCmd,
		"stat":  provideStatCmd,
	},
}

var provideClearCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Clear all CIDs from the provide queue.",
		ShortDescription: `
Clear all CIDs pending to be provided for the first time.

Note: Kubo will automatically clear the queue when it detects a change of
Provide.Strategy upon a restart. For more information about provide
strategies, see:
https://github.com/ipfs/kubo/blob/master/docs/config.md#providestrategy
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

type provideStats struct {
	provider.ReproviderStats
	fullRT bool
}

var provideStatCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Returns statistics about the node's provider system.",
		ShortDescription: `
Returns statistics about the content the node is reproviding every
Provide.DHT.Interval according to Provide.Strategy:
https://github.com/ipfs/kubo/blob/master/docs/config.md#provide

This interface is not stable and may change from release to release.

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

		provideSys, ok := nd.Provider.(provider.System)
		if !ok {
			return errors.New("stats not available with experimental sweeping provider (Provide.DHT.SweepEnabled=true)")
		}

		stats, err := provideSys.Stat()
		if err != nil {
			return err
		}
		_, fullRT := nd.DHTClient.(*fullrt.FullRT)

		if err := res.Emit(provideStats{stats, fullRT}); err != nil {
			return err
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s provideStats) error {
			wtr := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			defer wtr.Flush()

			fmt.Fprintf(wtr, "TotalReprovides:\t%s\n", humanNumber(s.TotalReprovides))
			fmt.Fprintf(wtr, "AvgReprovideDuration:\t%s\n", humanDuration(s.AvgReprovideDuration))
			fmt.Fprintf(wtr, "LastReprovideDuration:\t%s\n", humanDuration(s.LastReprovideDuration))
			if !s.LastRun.IsZero() {
				fmt.Fprintf(wtr, "LastReprovide:\t%s\n", humanTime(s.LastRun))
				if s.fullRT {
					fmt.Fprintf(wtr, "NextReprovide:\t%s\n", humanTime(s.LastRun.Add(s.ReprovideInterval)))
				}
			}
			return nil
		}),
	},
	Type: provideStats{},
}

func humanDuration(val time.Duration) string {
	return val.Truncate(time.Microsecond).String()
}

func humanTime(val time.Time) string {
	return val.Format("2006-01-02 15:04:05")
}

func humanNumber[T constraints.Float | constraints.Integer](n T) string {
	nf := float64(n)
	str := humanSI(nf, 0)
	fullStr := humanFull(nf, 0)
	if str != fullStr {
		return fmt.Sprintf("%s\t(%s)", str, fullStr)
	}
	return str
}

func humanSI(val float64, decimals int) string {
	v, unit := humanize.ComputeSI(val)
	return fmt.Sprintf("%s%s", humanFull(v, decimals), unit)
}

func humanFull(val float64, decimals int) string {
	return humanize.CommafWithDigits(val, decimals)
}
