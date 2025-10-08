package commands

import (
	"errors"
	"fmt"
	"io"
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
		compact, _ := req.Options[provideStatCompactOptionName].(bool)
		all, _ := req.Options[provideStatAllOptionName].(bool)

		if compact && !all {
			return fmt.Errorf("--compact flag requires --all flag")
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

			if s.Legacy != nil {
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

			if s.Sweep.Closed {
				fmt.Fprintf(wtr, "Provider is closed\n")
				return nil
			}
			all, _ := req.Options[provideStatAllOptionName].(bool)
			compact, _ := req.Options[provideStatCompactOptionName].(bool)
			connectivity, _ := req.Options[provideStatConnectivityOptionName].(bool)
			queues, _ := req.Options[provideStatQueuesOptionName].(bool)
			schedule, _ := req.Options[provideStatScheduleOptionName].(bool)
			network, _ := req.Options[provideStatNetworkOptionName].(bool)
			timings, _ := req.Options[provideStatTimingsOptionName].(bool)
			operations, _ := req.Options[provideStatOperationsOptionName].(bool)
			workers, _ := req.Options[provideStatWorkersOptionName].(bool)

			brief := !all && !connectivity && !queues && !schedule && !network && !timings && !operations && !workers

			// Compact mode: display stats in 2 columns with fixed column 1 width
			if all && compact {
				const col1Width = 34 // Fixed width for column 1
				var col1, col2 []string

				availableReservedBurst := max(0, s.Sweep.Workers.DedicatedBurst-s.Sweep.Workers.ActiveBurst)
				availableReservedPeriodic := max(0, s.Sweep.Workers.DedicatedPeriodic-s.Sweep.Workers.ActivePeriodic)
				availableFreeWorkers := s.Sweep.Workers.Max - max(s.Sweep.Workers.DedicatedBurst, s.Sweep.Workers.ActiveBurst) - max(s.Sweep.Workers.DedicatedPeriodic, s.Sweep.Workers.ActivePeriodic)
				availableBurst := availableFreeWorkers + availableReservedBurst
				availablePeriodic := availableFreeWorkers + availableReservedPeriodic

				// Column 1: Schedule
				col1 = append(col1, "Schedule:")
				col1 = append(col1, fmt.Sprintf("  CIDs scheduled: %s", humanNumber(s.Sweep.Schedule.Keys)))
				col1 = append(col1, fmt.Sprintf("  Regions scheduled: %s", humanNumberOrNA(s.Sweep.Schedule.Regions)))
				col1 = append(col1, fmt.Sprintf("  Avg prefix length: %s", humanNumberOrNA(s.Sweep.Schedule.AvgPrefixLength)))
				col1 = append(col1, fmt.Sprintf("  Next reprovide at: %s", s.Sweep.Schedule.NextReprovideAt.Format("15:04:05")))
				nextPrefix := key.BitString(s.Sweep.Schedule.NextReprovidePrefix)
				if nextPrefix == "" {
					nextPrefix = "N/A"
				}
				col1 = append(col1, fmt.Sprintf("  Next prefix: %s", nextPrefix))
				col1 = append(col1, "")

				// Column 1: Network
				col1 = append(col1, "Network:")
				col1 = append(col1, fmt.Sprintf("  Avg record holders: %s", humanFloatOrNA(s.Sweep.Network.AvgHolders)))
				col1 = append(col1, fmt.Sprintf("  Peers swept: %s", humanNumber(s.Sweep.Network.Peers)))
				if s.Sweep.Network.Peers > 0 {
					col1 = append(col1, fmt.Sprintf("  Reachable peers: %s (%s%%)", humanNumber(s.Sweep.Network.Reachable), humanNumber(100*s.Sweep.Network.Reachable/s.Sweep.Network.Peers)))
				} else {
					col1 = append(col1, fmt.Sprintf("  Reachable peers: %s", humanNumber(s.Sweep.Network.Reachable)))
				}
				col1 = append(col1, fmt.Sprintf("  Avg region size: %s", humanFloatOrNA(s.Sweep.Network.AvgRegionSize)))
				col1 = append(col1, fmt.Sprintf("  Full keyspace coverage: %t", s.Sweep.Network.CompleteKeyspaceCoverage))
				col1 = append(col1, fmt.Sprintf("  Replication factor: %s", humanNumber(s.Sweep.Network.ReplicationFactor)))
				col1 = append(col1, "")

				// Column 1: Workers
				col1 = append(col1, "Workers:")
				col1 = append(col1, fmt.Sprintf("  Active: %s / %s (max)", humanNumber(s.Sweep.Workers.Active), humanNumber(s.Sweep.Workers.Max)))
				col1 = append(col1, fmt.Sprintf("  Free: %s", humanNumber(availableFreeWorkers)))
				col1 = append(col1, fmt.Sprintf("  Worker stats:  %-9s %s", "Periodic", "Burst"))
				col1 = append(col1, fmt.Sprintf("    %-12s %-9s %s", "Active:", humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(s.Sweep.Workers.ActiveBurst)))
				col1 = append(col1, fmt.Sprintf("    %-12s %-9s %s", "Dedicated:", humanNumber(s.Sweep.Workers.DedicatedPeriodic), humanNumber(s.Sweep.Workers.DedicatedBurst)))
				col1 = append(col1, fmt.Sprintf("    %-12s %-9s %s", "Available:", humanNumber(availablePeriodic), humanNumber(availableBurst)))
				col1 = append(col1, fmt.Sprintf("    %-12s %-9s %s", "Queued:", humanNumber(s.Sweep.Workers.QueuedPeriodic), humanNumber(s.Sweep.Workers.QueuedBurst)))
				col1 = append(col1, fmt.Sprintf("  Max connections/worker: %s", humanNumber(s.Sweep.Workers.MaxProvideConnsPerWorker)))

				// Column 2: Connectivity
				col2 = append(col2, "Connectivity:")
				since := s.Sweep.Connectivity.Since
				if since.IsZero() {
					col2 = append(col2, fmt.Sprintf("  Status: %s", s.Sweep.Connectivity.Status))
				} else {
					col2 = append(col2, fmt.Sprintf("  Status: %s (%s)", s.Sweep.Connectivity.Status, humanTime(since)))
				}
				col2 = append(col2, "")

				// Column 2: Queues
				col2 = append(col2, "Queues:")
				col2 = append(col2, fmt.Sprintf("  Provide queue: %s CIDs, %s regions", humanNumber(s.Sweep.Queues.PendingKeyProvides), humanNumber(s.Sweep.Queues.PendingRegionProvides)))
				col2 = append(col2, fmt.Sprintf("  Reprovide queue: %s regions", humanNumber(s.Sweep.Queues.PendingRegionReprovides)))
				col2 = append(col2, "")

				// Column 2: Operations
				col2 = append(col2, "Operations:")
				col2 = append(col2, fmt.Sprintf("  Ongoing provides: %s CIDs, %s regions", humanNumber(s.Sweep.Operations.Ongoing.KeyProvides), humanNumber(s.Sweep.Operations.Ongoing.RegionProvides)))
				col2 = append(col2, fmt.Sprintf("  Ongoing reprovides: %s CIDs, %s regions", humanNumber(s.Sweep.Operations.Ongoing.KeyReprovides), humanNumber(s.Sweep.Operations.Ongoing.RegionReprovides)))
				col2 = append(col2, fmt.Sprintf("  Total CIDs provided: %s", humanNumber(s.Sweep.Operations.Past.KeysProvided)))
				col2 = append(col2, fmt.Sprintf("  Total records provided: %s", humanNumber(s.Sweep.Operations.Past.RecordsProvided)))
				col2 = append(col2, fmt.Sprintf("  Total provide errors: %s", humanNumber(s.Sweep.Operations.Past.KeysFailed)))
				col2 = append(col2, fmt.Sprintf("  CIDs provided/min: %s", humanFloatOrNA(s.Sweep.Operations.Past.KeysProvidedPerMinute)))
				col2 = append(col2, fmt.Sprintf("  CIDs reprovided/min: %s", humanFloatOrNA(s.Sweep.Operations.Past.KeysReprovidedPerMinute)))
				col2 = append(col2, fmt.Sprintf("  Region reprovide duration: %s", humanDurationOrNA(s.Sweep.Operations.Past.RegionReprovideDuration)))
				col2 = append(col2, fmt.Sprintf("  Avg CIDs/reprovide: %s", humanFloatOrNA(s.Sweep.Operations.Past.AvgKeysPerReprovide)))
				col2 = append(col2, fmt.Sprintf("  Regions reprovided (last cycle): %s", humanNumber(s.Sweep.Operations.Past.RegionReprovidedLastCycle)))
				col2 = append(col2, "")

				// Column 2: Timings
				col2 = append(col2, "Timings:")
				col2 = append(col2, fmt.Sprintf("  Uptime: %s (%s)", humanDuration(s.Sweep.Timing.Uptime), humanTime(time.Now().Add(-s.Sweep.Timing.Uptime))))
				col2 = append(col2, fmt.Sprintf("  Current time offset: %s", humanDuration(s.Sweep.Timing.CurrentTimeOffset)))
				col2 = append(col2, fmt.Sprintf("  Cycle started: %s", humanTime(s.Sweep.Timing.CycleStart)))
				col2 = append(col2, fmt.Sprintf("  Reprovide interval: %s", humanDuration(s.Sweep.Timing.ReprovidesInterval)))

				// Print both columns side by side
				maxRows := max(len(col1), len(col2))
				for i := range maxRows {
					var left, right string
					if i < len(col1) {
						left = col1[i]
					}
					if i < len(col2) {
						right = col2[i]
					}
					fmt.Fprintf(wtr, "%-*s %s\n", col1Width, left, right)
				}
				return nil
			}

			// Connectivity
			if all || connectivity || brief && s.Sweep.Connectivity.Status != "online" {
				if !brief {
					fmt.Fprintf(wtr, "Connectivity:\n")
				}
				since := s.Sweep.Connectivity.Since
				indent := ""
				if !brief {
					indent = "  "
				}
				if since.IsZero() {
					fmt.Fprintf(wtr, "%sStatus:\t%s\n", indent, s.Sweep.Connectivity.Status)
				} else {
					fmt.Fprintf(wtr, "%sStatus:\t%s (%s)\n", indent, s.Sweep.Connectivity.Status, humanTime(s.Sweep.Connectivity.Since))
				}
				if !brief {
					fmt.Fprintf(wtr, "\n")
				}
			}
			// Queues
			if all || queues || brief {
				if !brief {
					fmt.Fprintf(wtr, "Queues:\n")
				}
				indent := ""
				if !brief {
					indent = "  "
				}
				fmt.Fprintf(wtr, "%sProvide queue:\t%s CIDs,\t%s regions\n", indent, humanNumber(s.Sweep.Queues.PendingKeyProvides), humanNumber(s.Sweep.Queues.PendingRegionProvides))
				fmt.Fprintf(wtr, "%sReprovide queue:\t%s regions\n", indent, humanNumber(s.Sweep.Queues.PendingRegionReprovides))
				if !brief {
					fmt.Fprintf(wtr, "\n")
				}
			}
			// Schedule
			if all || schedule || brief {
				if !brief {
					fmt.Fprintf(wtr, "Schedule:\n")
				}
				indent := ""
				if !brief {
					indent = "  "
				}
				fmt.Fprintf(wtr, "%sCIDs scheduled:\t%s\n", indent, humanNumber(s.Sweep.Schedule.Keys))
				fmt.Fprintf(wtr, "%sRegions scheduled:\t%s\n", indent, humanNumberOrNA(s.Sweep.Schedule.Regions))
				if !brief {
					fmt.Fprintf(wtr, "%sAvg prefix length:\t%s\n", indent, humanNumberOrNA(s.Sweep.Schedule.AvgPrefixLength))
					fmt.Fprintf(wtr, "%sNext reprovide at:\t%s\n", indent, s.Sweep.Schedule.NextReprovideAt.Format("15:04:05"))
					nextPrefix := key.BitString(s.Sweep.Schedule.NextReprovidePrefix)
					if nextPrefix == "" {
						nextPrefix = "N/A"
					}
					fmt.Fprintf(wtr, "%sNext prefix:\t%s\n", indent, nextPrefix)
				}
				if !brief {
					fmt.Fprintf(wtr, "\n")
				}
			}
			// Timings
			if all || timings {
				fmt.Fprintf(wtr, "Timings:\n")
				fmt.Fprintf(wtr, "  Uptime:\t%s (%s)\n", humanDuration(s.Sweep.Timing.Uptime), humanTime(time.Now().Add(-s.Sweep.Timing.Uptime)))
				fmt.Fprintf(wtr, "  Current time offset:\t%s\n", humanDuration(s.Sweep.Timing.CurrentTimeOffset))
				fmt.Fprintf(wtr, "  Cycle started:\t%s\n", humanTime(s.Sweep.Timing.CycleStart))
				fmt.Fprintf(wtr, "  Reprovide interval:\t%s\n", humanDuration(s.Sweep.Timing.ReprovidesInterval))
				fmt.Fprintf(wtr, "\n")
			}
			// Network
			if all || network || brief {
				if !brief {
					fmt.Fprintf(wtr, "Network:\n")
				}
				indent := ""
				if !brief {
					indent = "  "
				}
				fmt.Fprintf(wtr, "%sAvg record holders:\t%s\n", indent, humanFloatOrNA(s.Sweep.Network.AvgHolders))
				if !brief {
					fmt.Fprintf(wtr, "%sPeers swept:\t%s\n", indent, humanNumber(s.Sweep.Network.Peers))
					if s.Sweep.Network.Peers > 0 {
						fmt.Fprintf(wtr, "%sReachable peers:\t%s (%s%%)\n", indent, humanNumber(s.Sweep.Network.Reachable), humanNumber(100*s.Sweep.Network.Reachable/s.Sweep.Network.Peers))
					} else {
						fmt.Fprintf(wtr, "%sReachable peers:\t%s\n", indent, humanNumber(s.Sweep.Network.Reachable))
					}
					fmt.Fprintf(wtr, "%sAvg region size:\t%s\n", indent, humanFloatOrNA(s.Sweep.Network.AvgRegionSize))
					fmt.Fprintf(wtr, "%sFull keyspace coverage:\t%t\n", indent, s.Sweep.Network.CompleteKeyspaceCoverage)
					fmt.Fprintf(wtr, "%sReplication factor:\t%s\n", indent, humanNumber(s.Sweep.Network.ReplicationFactor))
					fmt.Fprintf(wtr, "\n")
				}
			}
			// Operations
			if all || operations || brief {
				if !brief {
					fmt.Fprintf(wtr, "Operations:\n")
				}
				indent := ""
				if !brief {
					indent = "  "
				}
				// Ongoing operations
				fmt.Fprintf(wtr, "%sOngoing provides:\t%s CIDs,\t%s regions\n", indent, humanNumber(s.Sweep.Operations.Ongoing.KeyProvides), humanNumber(s.Sweep.Operations.Ongoing.RegionProvides))
				fmt.Fprintf(wtr, "%sOngoing reprovides:\t%s CIDs,\t%s regions\n", indent, humanNumber(s.Sweep.Operations.Ongoing.KeyReprovides), humanNumber(s.Sweep.Operations.Ongoing.RegionReprovides))
				// Past operations summary
				fmt.Fprintf(wtr, "%sTotal CIDs provided:\t%s\n", indent, humanNumber(s.Sweep.Operations.Past.KeysProvided))
				if !brief {
					fmt.Fprintf(wtr, "%sTotal records provided:\t%s\n", indent, humanNumber(s.Sweep.Operations.Past.RecordsProvided))
					fmt.Fprintf(wtr, "%sTotal provide errors:\t%s\n", indent, humanNumber(s.Sweep.Operations.Past.KeysFailed))
					fmt.Fprintf(wtr, "%sCIDs provided/min:\t%s\n", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysProvidedPerMinute))
					fmt.Fprintf(wtr, "%sCIDs reprovided/min:\t%s\n", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysReprovidedPerMinute))
					fmt.Fprintf(wtr, "%sRegion reprovide duration:\t%s\n", indent, humanDurationOrNA(s.Sweep.Operations.Past.RegionReprovideDuration))
					fmt.Fprintf(wtr, "%sAvg CIDs/reprovide:\t%s\n", indent, humanFloatOrNA(s.Sweep.Operations.Past.AvgKeysPerReprovide))
					fmt.Fprintf(wtr, "%sRegions reprovided (last cycle):\t%s\n", indent, humanNumber(s.Sweep.Operations.Past.RegionReprovidedLastCycle))
					fmt.Fprintf(wtr, "\n")
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
					if !brief && displayWorkers {
						fmt.Fprintf(wtr, "Workers:\n")
					}
					indent := ""
					if !brief && displayWorkers {
						indent = "  "
					}
					fmt.Fprintf(wtr, "%sActive:\t%s / %s (max)\n", indent, humanNumber(s.Sweep.Workers.Active), humanNumber(s.Sweep.Workers.Max))
					fmt.Fprintf(wtr, "%sFree:\t%s\n", indent, humanNumber(availableFreeWorkers))
					if !brief && displayWorkers {
						fmt.Fprintf(wtr, "%sWorker stats:    %-9s %s\n", indent, "Periodic", "Burst")
						fmt.Fprintf(wtr, "%s  %-14s %-9s %s\n", indent, "Active:", humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(s.Sweep.Workers.ActiveBurst))
						fmt.Fprintf(wtr, "%s  %-14s %-9s %s\n", indent, "Dedicated:", humanNumber(s.Sweep.Workers.DedicatedPeriodic), humanNumber(s.Sweep.Workers.DedicatedBurst))
						fmt.Fprintf(wtr, "%s  %-14s %-9s %s\n", indent, "Available:", humanNumber(availablePeriodic), humanNumber(availableBurst))
						fmt.Fprintf(wtr, "%s  %-14s %-9s %s\n", indent, "Queued:", humanNumber(s.Sweep.Workers.QueuedPeriodic), humanNumber(s.Sweep.Workers.QueuedBurst))
					} else {
						// Brief mode - show condensed worker info
						fmt.Fprintf(wtr, "%sPeriodic:\t%s active, %s available, %s queued\n", indent,
							humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(availablePeriodic), humanNumber(s.Sweep.Workers.QueuedPeriodic))
						fmt.Fprintf(wtr, "%sBurst:\t%s active, %s available, %s queued\n", indent,
							humanNumber(s.Sweep.Workers.ActiveBurst), humanNumber(availableBurst), humanNumber(s.Sweep.Workers.QueuedBurst))
					}
				}
				if displayWorkers {
					indent := ""
					if !brief {
						indent = "  "
					}
					fmt.Fprintf(wtr, "%sMax connections/worker:\t%s\n", indent, humanNumber(s.Sweep.Workers.MaxProvideConnsPerWorker))
					if !brief {
						fmt.Fprintf(wtr, "\n")
					}
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
