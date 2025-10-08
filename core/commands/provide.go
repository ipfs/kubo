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

		fmt.Printf("sweepging provider %v\n", sweepingProvider)
		s := sweepingProvider.Stats()
		fmt.Printf("stats %v\n", s)
		return res.Emit(provideStats{Sweep: &s, Legacy: nil, FullRT: false})
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

			fmt.Fprintf(wtr, "Provide Sweep Stats:\n\n")
			if s.Sweep.Closed {
				fmt.Fprintf(wtr, "Status:\tclosed\n")
				return nil
			}
			all, _ := req.Options[provideStatAllOptionName].(bool)
			connectivity, _ := req.Options[provideStatConnectivityOptionName].(bool)
			queues, _ := req.Options[provideStatQueuesOptionName].(bool)
			schedule, _ := req.Options[provideStatScheduleOptionName].(bool)
			network, _ := req.Options[provideStatNetworkOptionName].(bool)
			timings, _ := req.Options[provideStatTimingsOptionName].(bool)
			operations, _ := req.Options[provideStatOperationsOptionName].(bool)
			workers, _ := req.Options[provideStatWorkersOptionName].(bool)

			brief := !all && !connectivity && !queues && !schedule && !network && !timings && !operations && !workers

			// Connectivity
			if all || connectivity || brief && s.Sweep.Connectivity.Status != "online" {
				fmt.Fprintf(wtr, "Connectivity:\t%s, since:\t%s\n", s.Sweep.Connectivity.Status, humanTime(s.Sweep.Connectivity.Since))
			}
			// Queues
			if all || queues || brief {
				fmt.Fprintf(wtr, "Provide Queue Size:\t%s CIDs, from:\t%s keyspace regions\n", humanNumber(s.Sweep.Queues.PendingKeyProvides), humanNumber(s.Sweep.Queues.PendingRegionProvides))
				fmt.Fprintf(wtr, "Reprovide Queue Size:\t%s regions\n", humanNumber(s.Sweep.Queues.PendingRegionReprovides))
			}
			// Schedule
			if all || schedule || brief {
				fmt.Fprintf(wtr, "CIDs scheduled for reprovide:\t%s\n", humanNumber(s.Sweep.Schedule.Keys))
				fmt.Fprintf(wtr, "Regions scheduled for reprovide:\t%s\n", humanNumber(s.Sweep.Schedule.Regions))
				if !brief {
					fmt.Fprintf(wtr, "Avg Prefix Length:\t%s\n", humanNumber(s.Sweep.Schedule.AvgPrefixLength))
					fmt.Fprintf(wtr, "Next Reprovide at:\t%s\n", humanTime(s.Sweep.Schedule.NextReprovideAt))
					fmt.Fprintf(wtr, "Next Reprovide Prefix:\t%s\n", key.BitString(s.Sweep.Schedule.NextReprovidePrefix))
				}
			}
			// Timings
			if all || timings {
				fmt.Fprintf(wtr, "Uptime:\t%s, Since:\t%s\n", humanDuration(s.Sweep.Timing.Uptime), humanTime(time.Now().Add(-s.Sweep.Timing.Uptime)))
				fmt.Fprintf(wtr, "Current Time Offset:\t%s\n", humanDuration(s.Sweep.Timing.CurrentTimeOffset))
				fmt.Fprintf(wtr, "Cycle Started:\t%s\n", humanTime(s.Sweep.Timing.CycleStart))
				fmt.Fprintf(wtr, "Reprovides Interval:\t%s\n", humanDuration(s.Sweep.Timing.ReprovidesInterval))
			}
			// Network
			if all || network || brief {
				fmt.Fprintf(wtr, "Avg Record Holders:\t%.1f\n", s.Sweep.Network.AvgHolders)
				if !brief {
					fmt.Fprintf(wtr, "Peers Contacted:\t%s\n", humanNumber(s.Sweep.Network.Peers))
					fmt.Fprintf(wtr, "Reachable Peers:\t%s\t(%d%%)\n", humanNumber(s.Sweep.Network.Reachable), 100*s.Sweep.Network.Reachable/s.Sweep.Network.Peers)
					fmt.Fprintf(wtr, "Avg Region Size:\t%.f1\n", 0.) // TODO: add region size to stats
					fmt.Fprintf(wtr, "Full Keyspace Coverage:\t%t\n", s.Sweep.Network.CompleteKeyspaceCoverage)
					fmt.Fprintf(wtr, "Replication Factor:\t%s\n", humanNumber(s.Sweep.Network.ReplicationFactor))
				}
			}
			// Operations
			if all || operations || brief {
				fmt.Fprintf(wtr, "Currently Providing:\t%s CIDs, In:\t%s Regions\n", humanNumber(s.Sweep.Operations.Ongoing.KeyProvides), humanNumber(s.Sweep.Operations.Ongoing.RegionProvides))
				fmt.Fprintf(wtr, "Currently Repoviding:\t%s CIDs, In:\t%s Regions\n", humanNumber(s.Sweep.Operations.Ongoing.KeyReprovides), humanNumber(s.Sweep.Operations.Ongoing.RegionReprovides))
				fmt.Fprintf(wtr, "Total Provides:\t%s\n", humanNumber(s.Sweep.Operations.Past.KeysProvided))
				if !brief {
					fmt.Fprintf(wtr, "Total Records Provided:\t%s\n", humanNumber(s.Sweep.Operations.Past.RecordsProvided))
					fmt.Fprintf(wtr, "Total Provide Errors:\t%s\n", humanNumber(s.Sweep.Operations.Past.KeysFailed))
					fmt.Fprintf(wtr, "CIDs Provided per Minute:\t%s\n", humanNumber(s.Sweep.Operations.Past.KeysProvidedPerMinute))
					fmt.Fprintf(wtr, "CIDs Reprovided per Minute:\t%s\n", humanNumber(s.Sweep.Operations.Past.KeysReprovidedPerMinute))
					fmt.Fprintf(wtr, "Region Reprovide Duration:\t%s\n", humanDuration(s.Sweep.Operations.Past.RegionReprovideDuration))
					fmt.Fprintf(wtr, "Avg CIDs per Reprovide:\t%s\n", humanNumber(s.Sweep.Operations.Past.AvgKeysPerReprovide))
					fmt.Fprintf(wtr, "Regions reprovided last cycle:\t%s\n", humanNumber(s.Sweep.Operations.Past.RegionReprovidedLastCycle))
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
					fmt.Fprintf(wtr, "Active Workers:\t%s, Max:\t%s\n", humanNumber(s.Sweep.Workers.Active), humanNumber(s.Sweep.Workers.Max))
					fmt.Fprintf(wtr, "Available Free Worker:\t%s\n", humanNumber(availableFreeWorkers))
					fmt.Fprintf(wtr, "Active Periodic Workers:\t%s, Dedicated:\t%s, Available:\t%s, Queued:\t%s\n",
						humanNumber(s.Sweep.Workers.ActivePeriodic), humanNumber(s.Sweep.Workers.DedicatedPeriodic),
						humanNumber(availablePeriodic), humanNumber(s.Sweep.Workers.QueuedPeriodic))
					fmt.Fprintf(wtr, "Active Burst Workers:\t%s, Dedicated:\t%s, Available:\t%s, Queued:\t%s\n",
						humanNumber(s.Sweep.Workers.ActiveBurst), humanNumber(s.Sweep.Workers.DedicatedBurst),
						humanNumber(availableBurst), humanNumber(s.Sweep.Workers.QueuedBurst))
				}
				if displayWorkers {
					fmt.Fprintf(wtr, "Max Connections per Worker:\t%s\n", humanNumber(s.Sweep.Workers.MaxProvideConnsPerWorker))
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
