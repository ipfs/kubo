package commands

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/ipfs/boxo/provider"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"golang.org/x/exp/constraints"
)

var statProvideCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Returns statistics about the node's (re)provider system.",
		ShortDescription: `
Returns statistics about the content the node is advertising.

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

		stats, err := nd.Provider.Stat()
		if err != nil {
			return err
		}

		if err := res.Emit(stats); err != nil {
			return err
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s *provider.ReproviderStats) error {
			wtr := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			defer wtr.Flush()

			fmt.Fprintf(wtr, "TotalProvides:\t%s\n", humanNumber(s.TotalProvides))
			fmt.Fprintf(wtr, "AvgProvideDuration:\t%s\n", humanDuration(s.AvgProvideDuration))
			fmt.Fprintf(wtr, "LastReprovideDuration:\t%s\n", humanDuration(s.LastReprovideDuration))
			fmt.Fprintf(wtr, "LastReprovideBatchSize:\t%s\n", humanNumber(s.LastReprovideBatchSize))
			return nil
		}),
	},
	Type: provider.ReproviderStats{},
}

func humanDuration(val time.Duration) string {
	return val.Truncate(time.Microsecond).String()
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
