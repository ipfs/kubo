package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	boxoprovider "github.com/ipfs/boxo/provider"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"github.com/libp2p/go-libp2p-kad-dht/provider"
	"github.com/libp2p/go-libp2p-kad-dht/provider/buffered"
	"github.com/libp2p/go-libp2p-kad-dht/provider/dual"
	"github.com/libp2p/go-libp2p-kad-dht/provider/stats"
	"github.com/probe-lab/go-libdht/kad/key"
	"golang.org/x/exp/constraints"
)

const (
	provideQuietOptionName = "quiet"
	provideLanOptionName   = "lan"

	provideStatAllOptionName          = "all"
	provideStatCompactOptionName      = "compact"
	provideStatNetworkOptionName      = "network"
	provideStatConnectivityOptionName = "connectivity"
	provideStatOperationsOptionName   = "operations"
	provideStatTimingsOptionName      = "timings"
	provideStatScheduleOptionName     = "schedule"
	provideStatQueuesOptionName       = "queues"
	provideStatWorkersOptionName      = "workers"
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
	Sweep  *stats.Stats
	Legacy *boxoprovider.ReproviderStats
	FullRT bool // only used for legacy stats
}

var provideStatCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Returns statistics about the node's provider system.",
		ShortDescription: `
Returns statistics about the content the node is reproviding every
Provide.DHT.Interval according to Provide.Strategy:
https://github.com/ipfs/kubo/blob/master/docs/config.md#provide

This command displays statistics for the provide system currently in use
(Sweep or Legacy). If using the Legacy provider, basic statistics are shown
and no flags are supported. The following behavior applies to the Sweep
provider only:

By default, displays a brief summary of key metrics including queue sizes,
scheduled CIDs/regions, average record holders, ongoing/total provides, and
worker status (if low on workers).

Use --all to display comprehensive statistics organized into sections:
connectivity (DHT status), queues (pending provides/reprovides), schedule
(CIDs/regions to reprovide), timings (uptime, cycle info), network (peers,
reachability, region size), operations (provide rates, errors), and workers
(pool utilization).

Individual sections can be displayed using their respective flags (e.g.,
--network, --operations, --workers). Multiple section flags can be combined.

The --compact flag provides a 2-column layout suitable for monitoring with
'watch' (requires --all). Example: watch ipfs provide stat --all --compact

For Dual DHT setups, use --lan to show statistics for the LAN DHT provider
instead of the default WAN DHT provider.

This interface is not stable and may change from release to release.

`,
	},
	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.BoolOption(provideLanOptionName, "Show stats for LAN DHT only (for Sweep+Dual DHT only)"),
		cmds.BoolOption(provideStatAllOptionName, "a", "Display all provide sweep stats"),
		cmds.BoolOption(provideStatCompactOptionName, "Display stats in 2-column layout (requires --all)"),
		cmds.BoolOption(provideStatConnectivityOptionName, "Display DHT connectivity status"),
		cmds.BoolOption(provideStatNetworkOptionName, "Display network stats (peers, reachability, region size)"),
		cmds.BoolOption(provideStatScheduleOptionName, "Display reprovide schedule (CIDs/regions scheduled, next reprovide time)"),
		cmds.BoolOption(provideStatTimingsOptionName, "Display timing information (uptime, cycle start, reprovide interval)"),
		cmds.BoolOption(provideStatWorkersOptionName, "Display worker pool stats (active/available/queued workers)"),
		cmds.BoolOption(provideStatOperationsOptionName, "Display operation stats (ongoing/past provides, rates, errors)"),
		cmds.BoolOption(provideStatQueuesOptionName, "Display provide and reprovide queue sizes"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		lanStats, _ := req.Options[provideLanOptionName].(bool)

		if lanStats {
			if _, ok := nd.Provider.(*dual.SweepingProvider); !ok {
				return errors.New("LAN DHT stats only available for Sweep+Dual DHT")
			}
		}

		var sweepingProvider *provider.SweepingProvider
		switch prov := nd.Provider.(type) {
		case boxoprovider.System:
			stats, err := prov.Stat()
			if err != nil {
				return err
			}
			_, fullRT := nd.DHTClient.(*fullrt.FullRT)
			return res.Emit(provideStats{Legacy: &stats, FullRT: fullRT})
		case *provider.SweepingProvider:
			sweepingProvider = prov
		case *dual.SweepingProvider:
			if lanStats {
				sweepingProvider = prov.LAN
			} else {
				sweepingProvider = prov.WAN
			}
		case *buffered.SweepingProvider:
			switch inner := prov.Provider.(type) {
			case *provider.SweepingProvider:
				sweepingProvider = inner
			case *dual.SweepingProvider:
				if lanStats {
					sweepingProvider = inner.LAN
				} else {
					sweepingProvider = inner.WAN
				}
			default:
			}
		default:
		}
		if sweepingProvider == nil {
			return fmt.Errorf("stats not available with current routing system %T", nd.Provider)
		}

		s := sweepingProvider.Stats()
		return res.Emit(provideStats{Sweep: &s})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s provideStats) error {
			wtr := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			defer wtr.Flush()

			all, _ := req.Options[provideStatAllOptionName].(bool)
			compact, _ := req.Options[provideStatCompactOptionName].(bool)
			connectivity, _ := req.Options[provideStatConnectivityOptionName].(bool)
			queues, _ := req.Options[provideStatQueuesOptionName].(bool)
			schedule, _ := req.Options[provideStatScheduleOptionName].(bool)
			network, _ := req.Options[provideStatNetworkOptionName].(bool)
			timings, _ := req.Options[provideStatTimingsOptionName].(bool)
			operations, _ := req.Options[provideStatOperationsOptionName].(bool)
			workers, _ := req.Options[provideStatWorkersOptionName].(bool)

			flagCount := 0
			for _, b := range []bool{all, connectivity, queues, schedule, network, timings, operations, workers} {
				if b {
					flagCount++
				}
			}

			if s.Legacy != nil {
				if flagCount > 0 {
					return errors.New("cannot use flags with legacy provide stats")
				}
				fmt.Fprintf(wtr, "TotalReprovides:\t%s\n", humanNumber(s.Legacy.TotalReprovides))
				fmt.Fprintf(wtr, "AvgReprovideDuration:\t%s\n", humanDuration(s.Legacy.AvgReprovideDuration))
				fmt.Fprintf(wtr, "LastReprovideDuration:\t%s\n", humanDuration(s.Legacy.LastReprovideDuration))
				if !s.Legacy.LastRun.IsZero() {
					fmt.Fprintf(wtr, "LastReprovide:\t%s\n", humanTime(s.Legacy.LastRun))
					if s.FullRT {
						fmt.Fprintf(wtr, "NextReprovide:\t%s\n", humanTime(s.Legacy.LastRun.Add(s.Legacy.ReprovideInterval)))
					}
				}
				return nil
			}

			if s.Sweep == nil {
				return errors.New("no provide stats available")
			}

			// Sweep provider stats
			if s.Sweep.Closed {
				fmt.Fprintf(wtr, "Provider is closed\n")
				return nil
			}

			if compact && !all {
				return errors.New("--compact flag requires --all flag")
			}

			brief := flagCount == 0
			showHeadings := flagCount > 1 || all

			compactMode := all && compact
			var cols [2][]string
			col0MaxWidth := 0
			formatLine := func(col int, format string, a ...any) {
				if compactMode {
					s := fmt.Sprintf(format, a...)
					cols[col] = append(cols[col], s)
					col0MaxWidth = max(col0MaxWidth, len(s))
					return
				}
				format = strings.Replace(format, ": ", ":\t", 1)
				format = strings.Replace(format, ", ", ",\t", 1)
				cols[0] = append(cols[0], fmt.Sprintf(format, a...))
			}
			addBlankLine := func(col int) {
				if !brief {
					formatLine(col, "")
				}
			}
			sectionTitle := func(col int, title string) {
				if !brief && showHeadings {
					formatLine(col, title+":")
				}
			}

			indent := "  "
			if brief || !showHeadings {
				indent = ""
			}

			// Connectivity
			if all || connectivity || brief && s.Sweep.Connectivity.Status != "online" {
				sectionTitle(1, "Connectivity")
				since := s.Sweep.Connectivity.Since
				if since.IsZero() {
					formatLine(1, "%sStatus: %s", indent, s.Sweep.Connectivity.Status)
				} else {
					formatLine(1, "%sStatus: %s (%s)", indent, s.Sweep.Connectivity.Status, humanTime(since))
				}
				addBlankLine(1)
			}

			// Queues
			if all || queues || brief {
				sectionTitle(1, "Queues")
				formatLine(1, "%sProvide queue: %s CIDs, %s regions", indent, humanNumber(s.Sweep.Queues.PendingKeyProvides), humanNumber(s.Sweep.Queues.PendingRegionProvides))
				formatLine(1, "%sReprovide queue: %s regions", indent, humanNumber(s.Sweep.Queues.PendingRegionReprovides))
				addBlankLine(1)
			}

			// Schedule
			if all || schedule || brief {
				sectionTitle(0, "Schedule")
				formatLine(0, "%sCIDs scheduled: %s", indent, humanNumber(s.Sweep.Schedule.Keys))
				formatLine(0, "%sRegions scheduled: %s", indent, humanNumberOrNA(s.Sweep.Schedule.Regions))
				if !brief {
					formatLine(0, "%sAvg prefix length: %s", indent, humanFloatOrNA(s.Sweep.Schedule.AvgPrefixLength))
					nextReprovideAt := s.Sweep.Schedule.NextReprovideAt.Format("15:04:05")
					if s.Sweep.Schedule.NextReprovideAt.IsZero() {
						nextReprovideAt = "N/A"
					}
					formatLine(0, "%sNext reprovide at: %s", indent, nextReprovideAt)
					nextPrefix := key.BitString(s.Sweep.Schedule.NextReprovidePrefix)
					if nextPrefix == "" {
						nextPrefix = "N/A"
					}
					formatLine(0, "%sNext prefix: %s", indent, nextPrefix)
				}
				addBlankLine(0)
			}

			// Timings
			if all || timings {
				sectionTitle(1, "Timings")
				formatLine(1, "%sUptime: %s (%s)", indent, humanDuration(s.Sweep.Timing.Uptime), humanTime(time.Now().Add(-s.Sweep.Timing.Uptime)))
				formatLine(1, "%sCurrent time offset: %s", indent, humanDuration(s.Sweep.Timing.CurrentTimeOffset))
				formatLine(1, "%sCycle started: %s", indent, humanTime(s.Sweep.Timing.CycleStart))
				formatLine(1, "%sReprovide interval: %s", indent, humanDuration(s.Sweep.Timing.ReprovidesInterval))
				addBlankLine(1)
			}

			// Network
			if all || network || brief {
				sectionTitle(0, "Network")
				formatLine(0, "%sAvg record holders: %s", indent, humanFloatOrNA(s.Sweep.Network.AvgHolders))
				if !brief {
					formatLine(0, "%sPeers swept: %s", indent, humanNumber(s.Sweep.Network.Peers))
					if s.Sweep.Network.Peers > 0 {
						formatLine(0, "%sReachable peers: %s (%s%%)", indent, humanNumber(s.Sweep.Network.Reachable), humanNumber(100*s.Sweep.Network.Reachable/s.Sweep.Network.Peers))
					} else {
						formatLine(0, "%sReachable peers: %s", indent, humanNumber(s.Sweep.Network.Reachable))
					}
					formatLine(0, "%sAvg region size: %s", indent, humanFloatOrNA(s.Sweep.Network.AvgRegionSize))
					formatLine(0, "%sFull keyspace coverage: %t", indent, s.Sweep.Network.CompleteKeyspaceCoverage)
					formatLine(0, "%sReplication factor: %s", indent, humanNumber(s.Sweep.Network.ReplicationFactor))
					addBlankLine(0)
				}
			}

			// Operations
			if all || operations || brief {
				sectionTitle(1, "Operations")
				// Ongoing operations
				formatLine(1, "%sOngoing provides: %s CIDs, %s regions", indent, humanNumber(s.Sweep.Operations.Ongoing.KeyProvides), humanNumber(s.Sweep.Operations.Ongoing.RegionProvides))
				formatLine(1, "%sOngoing reprovides: %s CIDs, %s regions", indent, humanNumber(s.Sweep.Operations.Ongoing.KeyReprovides), humanNumber(s.Sweep.Operations.Ongoing.RegionReprovides))
				// Past operations summary
				formatLine(1, "%sTotal CIDs provided: %s", indent, humanNumber(s.Sweep.Operations.Past.KeysProvided))
				if !brief {
					formatLine(1, "%sTotal records provided: %s", indent, humanNumber(s.Sweep.Operations.Past.RecordsProvided))
					formatLine(1, "%sTotal provide errors: %s", indent, humanNumber(s.Sweep.Operations.Past.KeysFailed))
					formatLine(1, "%sCIDs provided/min: %s", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysProvidedPerMinute))
					formatLine(1, "%sCIDs reprovided/min: %s", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysReprovidedPerMinute))
					formatLine(1, "%sRegion reprovide duration: %s", indent, humanDurationOrNA(s.Sweep.Operations.Past.RegionReprovideDuration))
					formatLine(1, "%sAvg CIDs/reprovide: %s", indent, humanFloatOrNA(s.Sweep.Operations.Past.AvgKeysPerReprovide))
					formatLine(1, "%sRegions reprovided (last cycle): %s", indent, humanNumber(s.Sweep.Operations.Past.RegionReprovidedLastCycle))
					addBlankLine(1)
				}
			}

			// Workers
			displayWorkers := all || workers
			if displayWorkers || brief {
				availableReservedBurst := max(0, s.Sweep.Workers.DedicatedBurst-s.Sweep.Workers.ActiveBurst)
				availableReservedPeriodic := max(0, s.Sweep.Workers.DedicatedPeriodic-s.Sweep.Workers.ActivePeriodic)
				availableFreeWorkers := s.Sweep.Workers.Max - max(s.Sweep.Workers.DedicatedBurst, s.Sweep.Workers.ActiveBurst) - max(s.Sweep.Workers.DedicatedPeriodic, s.Sweep.Workers.ActivePeriodic)
				availableBurst := availableFreeWorkers + availableReservedBurst
				availablePeriodic := availableFreeWorkers + availableReservedPeriodic

				if displayWorkers || availableBurst <= 2 || availablePeriodic <= 2 {
					// Either we want to display workers information, or we are low on
					// available workers and want to warn the user.
					sectionTitle(0, "Workers")
					specifyWorkers := " workers"
					if compactMode {
						specifyWorkers = ""
					}
					formatLine(0, "%sActive%s: %s / %s (max)", indent, specifyWorkers, humanNumber(s.Sweep.Workers.Active), humanNumber(s.Sweep.Workers.Max))
					if brief {
						// Brief mode - show condensed worker info
						formatLine(0, "%sPeriodic%s: %s active, %s available, %s queued", indent, specifyWorkers,
							humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(availablePeriodic), humanNumber(s.Sweep.Workers.QueuedPeriodic))
						formatLine(0, "%sBurst%s: %s active, %s available, %s queued\n", indent, specifyWorkers,
							humanNumber(s.Sweep.Workers.ActiveBurst), humanNumber(availableBurst), humanNumber(s.Sweep.Workers.QueuedBurst))
					} else {
						formatLine(0, "%sFree%s: %s", indent, specifyWorkers, humanNumber(availableFreeWorkers))
						formatLine(0, "%sWorker stats:%s  %-9s %s", indent, "  ", "Periodic", "Burst")
						formatLine(0, "%s  %-14s %-9s %s", indent, "Active:", humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(s.Sweep.Workers.ActiveBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Dedicated:", humanNumber(s.Sweep.Workers.DedicatedPeriodic), humanNumber(s.Sweep.Workers.DedicatedBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Available:", humanNumber(availablePeriodic), humanNumber(availableBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Queued:", humanNumber(s.Sweep.Workers.QueuedPeriodic), humanNumber(s.Sweep.Workers.QueuedBurst))
						formatLine(0, "%sMax connections/worker: %s", indent, humanNumber(s.Sweep.Workers.MaxProvideConnsPerWorker))
						addBlankLine(0)
					}
				}
			}
			if compactMode {
				col0Width := col0MaxWidth + 2
				// Print both columns side by side
				maxRows := max(len(cols[0]), len(cols[1]))
				for i := range maxRows - 1 { // last line is empty
					var left, right string
					if i < len(cols[0]) {
						left = cols[0][i]
					}
					if i < len(cols[1]) {
						right = cols[1][i]
					}
					fmt.Fprintf(wtr, "%-*s %s\n", col0Width, left, right)
				}
			} else {
				if !brief {
					cols[0] = cols[0][:len(cols[0])-1] // remove last blank line
				}
				for _, line := range cols[0] {
					fmt.Fprintln(wtr, line)
				}
			}
			return nil
		}),
	},
	Type: provideStats{},
}

func humanDuration(val time.Duration) string {
	if val > time.Second {
		return val.Truncate(100 * time.Millisecond).String()
	}
	return val.Truncate(time.Microsecond).String()
}

func humanDurationOrNA(val time.Duration) string {
	if val <= 0 {
		return "N/A"
	}
	return humanDuration(val)
}

func humanTime(val time.Time) string {
	if val.IsZero() {
		return "N/A"
	}
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

// humanNumberOrNA is like humanNumber but returns "N/A" for non-positive values.
func humanNumberOrNA[T constraints.Float | constraints.Integer](n T) string {
	if n <= 0 {
		return "N/A"
	}
	return humanNumber(n)
}

func humanFloatOrNA(val float64) string {
	if val <= 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f", val)
}

func humanSI(val float64, decimals int) string {
	v, unit := humanize.ComputeSI(val)
	return fmt.Sprintf("%s%s", humanFull(v, decimals), unit)
}

func humanFull(val float64, decimals int) string {
	return humanize.CommafWithDigits(val, decimals)
}
