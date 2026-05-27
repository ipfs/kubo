package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	humanize "github.com/dustin/go-humanize"
	"github.com/ipfs/boxo/dag/walker"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	boxoprovider "github.com/ipfs/boxo/provider"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"github.com/libp2p/go-libp2p-kad-dht/provider"
	"github.com/libp2p/go-libp2p-kad-dht/provider/buffered"
	"github.com/libp2p/go-libp2p-kad-dht/provider/dual"
	"github.com/libp2p/go-libp2p-kad-dht/provider/stats"
	routing "github.com/libp2p/go-libp2p/core/routing"
	"github.com/probe-lab/go-libdht/kad/key"
	"golang.org/x/exp/constraints"
	"golang.org/x/term"
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

	// lowWorkerThreshold is the threshold below which worker availability warnings are shown
	lowWorkerThreshold = 2
)

var ProvideCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Control and monitor content providing",
		ShortDescription: `
Control providing operations.

OVERVIEW:

The provide system publishes provider records so other peers can discover
which nodes hold each CID. Content is reprovided periodically (every
Provide.DHT.Interval) according to Provide.Strategy.

CONFIGURATION:

Learn more: https://github.com/ipfs/kubo/blob/master/docs/config.md#provide

SEE ALSO:

For ad-hoc immediate announcements, see 'ipfs provide once'.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"clear": provideClearCmd,
		"once":  provideOnceCmd,
		"stat":  provideStatCmd,
	},
}

var provideClearCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Clear all CIDs from the provide queue.",
		ShortDescription: `
Clears the provide queue: CIDs waiting to be advertised to the DHT for the
first time. Does not affect content that is already being reprovided on
schedule.

Kubo also clears the queue automatically on restart when it detects a
change of Provide.Strategy.

See: https://github.com/ipfs/kubo/blob/master/docs/config.md#providestrategy
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

// ProvideOnceEvent is emitted once per CID announced by 'ipfs provide once'.
type ProvideOnceEvent struct {
	Queued string
}

var provideOnceCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Announce CIDs to the routing system on demand.",
		ShortDescription: `
Publishes provider records for the given CIDs once. The periodic
reprovide schedule (driven by Provide.Strategy and Provide.DHT.Interval)
is left unchanged: CIDs announced here are NOT added to the schedule.
CIDs can be passed as arguments or streamed from stdin (one per line).

The default sweep provider (Provide.DHT.SweepEnabled=true) submits the CIDs
to its burst-provide queue and returns as each CID is queued; dedicated
burst workers publish the records to the DHT. Use 'ipfs provide stat' to
monitor progress.

The legacy provider (Provide.DHT.SweepEnabled=false) queues the CIDs for
its serial worker pool, which publishes one CID at a time and may take
significantly longer to complete.

Use --recursive to walk the DAG and announce every reachable block. With
the default Provide.Strategy=all, every block is already announced, so -r
is only useful with selective strategies like 'roots' or 'pinned+entities'.

CIDs must already exist in the local blockstore.

CIDs are deduplicated across arguments, stdin, and DAG walks. Dedup uses
a bloom filter, so at very large scale a small fraction of CIDs may be
skipped (default rate ~1 in 4.75M).

OUTPUT:

Output is streamed as each CID is queued. With --enc=json, one
{"Queued": "<cid>"} object is emitted per line. With the text encoder
(default) on a terminal, a single line shows the running count; on a pipe,
a final count is printed at the end.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "The CID(s) to announce.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(recursiveOptionName, "r", "Recursively announce the entire DAG."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		if !nd.IsOnline {
			return ErrNotOnline
		}
		cfg, err := nd.Repo.Config()
		if err != nil {
			return err
		}
		if !cfg.Provide.Enabled.WithDefault(config.DefaultProvideEnabled) {
			return errors.New("cannot provide: Provide.Enabled is false")
		}
		if len(nd.PeerHost.Network().Conns()) == 0 && !cfg.HasHTTPProviderConfigured() {
			return errors.New("cannot provide: no connected peers")
		}

		recursive, _ := req.Options[recursiveOptionName].(bool)

		// seen deduplicates across all roots and recursive walks, so a CID
		// shared by multiple roots (or repeated in argv/stdin) is announced
		// exactly once per invocation. The bloom autoscales as more CIDs
		// arrive, keeping memory bounded for arbitrarily large inputs at
		// the cost of a small false-positive rate (default ~1 in 4.75M)
		// that may cause an occasional CID to be skipped.
		seen, err := walker.NewBloomTracker(walker.MinBloomCapacity, walker.DefaultBloomFPRate)
		if err != nil {
			return err
		}

		// announce queues a single CID into the provide system and emits one
		// event for it. Uses ProvideOnce so the CID is published without
		// being added to the keystore: the periodic reprovide schedule
		// (driven by Provide.Strategy) is unaffected. Errors propagate to
		// the caller.
		announce := func(c cid.Cid) error {
			if err := nd.Provider.ProvideOnce(c.Hash()); err != nil {
				return err
			}
			return res.Emit(&ProvideOnceEvent{Queued: c.String()})
		}

		// processRoot validates a root CID against the local blockstore and
		// announces either just that CID or every block reachable from it.
		processRoot := func(arg string) error {
			c, err := cid.Decode(arg)
			if err != nil {
				return fmt.Errorf("invalid CID %q: %w", arg, err)
			}
			has, err := nd.Blockstore.Has(req.Context, c)
			if err != nil {
				return err
			}
			if !has {
				return fmt.Errorf("block %s not found locally, cannot provide", c)
			}

			if !recursive {
				if !seen.Visit(c) {
					return nil
				}
				return announce(c)
			}

			// Stream per-block: visit emits as it walks. Cancel the walk on
			// the first announce error so we don't keep fetching DAG nodes
			// after we've already failed.
			ctx, cancel := context.WithCancel(req.Context)
			defer cancel()
			var visitErr error
			walkErr := dag.Walk(ctx, dag.GetLinksDirect(nd.DAG), c, func(child cid.Cid) bool {
				// Skip subtrees we've already walked from a previous root or
				// argument: returning false stops descent into this node.
				if !seen.Visit(child) {
					return false
				}
				if err := announce(child); err != nil {
					visitErr = err
					cancel()
					return false
				}
				return true
			})
			if visitErr != nil {
				return visitErr
			}
			return walkErr
		}

		args := argumentIterator{req.Arguments, req.BodyArgs()}
		for {
			arg, ok := args.next()
			if !ok {
				break
			}
			if err := processRoot(arg); err != nil {
				return err
			}
		}
		return args.err()
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			// In text mode we render the running counter and final summary
			// directly to stderr/stdout, bypassing the encoder so the TTY
			// redraw works. For other encoders (json, xml) we must let the
			// encoder serialize each event, so forward the stream as-is.
			if enc, _ := res.Request().Options[cmds.EncLong].(string); enc != "" && enc != cmds.Text {
				return cmds.Copy(re, res)
			}

			// Text mode: render directly to stderr/stdout below. Do not
			// call re.Emit from this branch, or output will race with the
			// running counter.
			isTTY := term.IsTerminal(int(os.Stderr.Fd()))
			var count int
			for {
				v, err := res.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					if isTTY && count > 0 {
						fmt.Fprintln(os.Stderr)
					}
					return err
				}
				if _, ok := v.(*ProvideOnceEvent); !ok {
					log.Errorf("provide once postrun: received unexpected type %T", v)
					continue
				}
				count++
				if isTTY {
					fmt.Fprintf(os.Stderr, "\rqueued %d CID(s) for immediate provide", count)
				}
			}
			if isTTY && count > 0 {
				fmt.Fprintln(os.Stderr)
			} else {
				fmt.Fprintf(os.Stdout, "queued %d CID(s) for immediate provide\n", count)
			}
			return nil
		},
	},
	Type: ProvideOnceEvent{},
	Encoders: cmds.EncoderMap{
		// Used when PostRun is not invoked (HTTP API consumers in text mode).
		// One CID per line keeps the stream pipe-friendly.
		cmds.Text: cmds.MakeTypedEncoder(func(_ *cmds.Request, w io.Writer, e *ProvideOnceEvent) error {
			_, err := fmt.Fprintf(w, "%s\n", e.Queued)
			return err
		}),
	},
}

type provideStats struct {
	Sweep  *stats.Stats
	Legacy *boxoprovider.ReproviderStats
	FullRT bool // only used for legacy stats
}

// extractSweepingProvider extracts a SweepingProvider from the given provider interface.
// It handles unwrapping buffered and dual providers, selecting LAN or WAN as specified.
// Returns nil if the provider is not a sweeping provider type.
func extractSweepingProvider(prov any, useLAN bool) *provider.SweepingProvider {
	switch p := prov.(type) {
	case *provider.SweepingProvider:
		return p
	case *dual.SweepingProvider:
		if useLAN {
			return p.LAN
		}
		return p.WAN
	case *buffered.SweepingProvider:
		// Recursively extract from the inner provider
		return extractSweepingProvider(p.Provider, useLAN)
	default:
		return nil
	}
}

var provideStatCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Show statistics about the provide system",
		ShortDescription: `
Returns statistics about the node's provide system.

OVERVIEW:

The provide system publishes provider records mapping CIDs to your peer
ID. Records expire after a fixed TTL, so the system reprovides them on a
schedule to keep content discoverable.

Two provider types exist:

- Sweep provider (default): divides the DHT keyspace into regions and
  sweeps through them over the reprovide interval. Batches CIDs that map
  to the same DHT servers, reducing lookups from N (one per CID) to a
  small constant based on DHT size (~3k for 10k DHT servers). Spreads work
  evenly over time and announces records just before they expire.

- Legacy provider: announces each CID with a separate DHT lookup. Tries
  to reprovide all content as fast as possible at each cycle start. Fine
  for small datasets, slow past a few thousand CIDs.

Learn more:
- Config: https://github.com/ipfs/kubo/blob/master/docs/config.md#provide
- Metrics: https://github.com/ipfs/kubo/blob/master/docs/provide-stats.md

DEFAULT OUTPUT:

Shows a brief summary including queue sizes, scheduled items, average record
holders, ongoing/total provides, and worker warnings.

DETAILED OUTPUT:

Use --all for detailed statistics with these sections: connectivity, queues,
schedule, timings, network, operations, and workers. Individual sections can
be displayed with their flags (e.g., --network, --operations). Multiple flags
can be combined.

Use --compact for monitoring-friendly 2-column output (requires --all).

EXAMPLES:

Monitor provider statistics in real-time with 2-column layout:

  watch ipfs provide stat --all --compact

Get statistics in JSON format for programmatic processing:

  ipfs provide stat --enc=json | jq

NOTES:

- This interface is experimental and may change between releases
- Legacy provider shows basic stats only (no flags supported)
- "Regions" are keyspace divisions for spreading reprovide work
- For Dual DHT: use --lan for LAN provider stats (default is WAN)
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

		// Handle legacy provider
		if legacySys, ok := nd.Provider.(boxoprovider.System); ok {
			if lanStats {
				return errors.New("LAN stats only available for Sweep provider with Dual DHT")
			}
			stats, err := legacySys.Stat()
			if err != nil {
				return err
			}
			_, fullRT := nd.DHTClient.(*fullrt.FullRT)
			return res.Emit(provideStats{Legacy: &stats, FullRT: fullRT})
		}

		// Extract sweeping provider (handles buffered and dual unwrapping)
		sweepingProvider := extractSweepingProvider(nd.Provider, lanStats)
		if sweepingProvider == nil {
			if lanStats {
				return errors.New("LAN stats only available for Sweep provider with Dual DHT")
			}
			return fmt.Errorf("stats not available with current routing system %T", nd.Provider)
		}

		s, err := sweepingProvider.Stats(req.Context)
		if err != nil {
			return err
		}
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
			for _, enabled := range []bool{all, connectivity, queues, schedule, network, timings, operations, workers} {
				if enabled {
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
				return errors.New("--compact requires --all flag")
			}

			brief := flagCount == 0
			showHeadings := flagCount > 1 || all

			compactMode := all && compact
			var cols [2][]string
			col0MaxWidth := 0
			// formatLine handles both normal and compact output modes:
			// - Normal mode: all lines go to cols[0], col parameter is ignored
			// - Compact mode: col 0 for left column, col 1 for right column
			formatLine := func(col int, format string, a ...any) {
				if compactMode {
					s := fmt.Sprintf(format, a...)
					cols[col] = append(cols[col], s)
					if col == 0 {
						col0MaxWidth = max(col0MaxWidth, utf8.RuneCountInString(s))
					}
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
					formatLine(col, "%s:", title)
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
				formatLine(1, "%sProvide queue: %s CIDs, %s regions", indent, humanSI(s.Sweep.Queues.PendingKeyProvides, 1), humanSI(s.Sweep.Queues.PendingRegionProvides, 1))
				formatLine(1, "%sReprovide queue: %s regions", indent, humanSI(s.Sweep.Queues.PendingRegionReprovides, 1))
				addBlankLine(1)
			}

			// Schedule
			if all || schedule || brief {
				sectionTitle(0, "Schedule")
				formatLine(0, "%sCIDs scheduled: %s", indent, humanNumber(s.Sweep.Schedule.Keys))
				formatLine(0, "%sRegions scheduled: %s", indent, humanNumberOrNA(s.Sweep.Schedule.Regions))
				if !brief {
					formatLine(0, "%sAvg prefix length: %s", indent, humanFloatOrNA(s.Sweep.Schedule.AvgPrefixLength))
					nextPrefix := key.BitString(s.Sweep.Schedule.NextReprovidePrefix)
					if nextPrefix == "" {
						nextPrefix = "N/A"
					}
					formatLine(0, "%sNext region prefix: %s", indent, nextPrefix)
					nextReprovideAt := s.Sweep.Schedule.NextReprovideAt.Format("15:04:05")
					if s.Sweep.Schedule.NextReprovideAt.IsZero() {
						nextReprovideAt = "N/A"
					}
					formatLine(0, "%sNext region reprovide: %s", indent, nextReprovideAt)
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
					formatLine(0, "%sPeers swept: %s", indent, humanInt(s.Sweep.Network.Peers))
					formatLine(0, "%sFull keyspace coverage: %t", indent, s.Sweep.Network.CompleteKeyspaceCoverage)
					if s.Sweep.Network.Peers > 0 {
						formatLine(0, "%sReachable peers: %s (%s%%)", indent, humanInt(s.Sweep.Network.Reachable), humanNumber(100*s.Sweep.Network.Reachable/s.Sweep.Network.Peers))
					} else {
						formatLine(0, "%sReachable peers: %s", indent, humanInt(s.Sweep.Network.Reachable))
					}
					formatLine(0, "%sAvg region size: %s", indent, humanFloatOrNA(s.Sweep.Network.AvgRegionSize))
					formatLine(0, "%sReplication factor: %s", indent, humanNumber(s.Sweep.Network.ReplicationFactor))
					addBlankLine(0)
				}
			}

			// Operations
			if all || operations || brief {
				sectionTitle(1, "Operations")
				// Ongoing operations
				formatLine(1, "%sOngoing provides: %s CIDs, %s regions", indent, humanSI(s.Sweep.Operations.Ongoing.KeyProvides, 1), humanSI(s.Sweep.Operations.Ongoing.RegionProvides, 1))
				formatLine(1, "%sOngoing reprovides: %s CIDs, %s regions", indent, humanSI(s.Sweep.Operations.Ongoing.KeyReprovides, 1), humanSI(s.Sweep.Operations.Ongoing.RegionReprovides, 1))
				// Past operations summary
				formatLine(1, "%sTotal CIDs provided: %s", indent, humanNumber(s.Sweep.Operations.Past.KeysProvided))
				if !brief {
					formatLine(1, "%sTotal records provided: %s", indent, humanNumber(s.Sweep.Operations.Past.RecordsProvided))
					formatLine(1, "%sTotal provide errors: %s", indent, humanNumber(s.Sweep.Operations.Past.KeysFailed))
					formatLine(1, "%sCIDs provided/min/worker: %s", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysProvidedPerMinute))
					formatLine(1, "%sCIDs reprovided/min/worker: %s", indent, humanFloatOrNA(s.Sweep.Operations.Past.KeysReprovidedPerMinute))
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
				availableFreeWorkers := max(0, s.Sweep.Workers.Max-max(s.Sweep.Workers.DedicatedBurst, s.Sweep.Workers.ActiveBurst)-max(s.Sweep.Workers.DedicatedPeriodic, s.Sweep.Workers.ActivePeriodic))
				availableBurst := availableFreeWorkers + availableReservedBurst
				availablePeriodic := availableFreeWorkers + availableReservedPeriodic

				if displayWorkers || availableBurst <= lowWorkerThreshold || availablePeriodic <= lowWorkerThreshold {
					// Either we want to display workers information, or we are low on
					// available workers and want to warn the user.
					sectionTitle(0, "Workers")
					specifyWorkers := " workers"
					if compactMode {
						specifyWorkers = ""
					}
					formatLine(0, "%sActive%s: %s / %s (max)", indent, specifyWorkers, humanInt(s.Sweep.Workers.Active), humanInt(s.Sweep.Workers.Max))
					if brief {
						// Brief mode - show condensed worker info
						formatLine(0, "%sPeriodic%s: %s active, %s available, %s queued", indent, specifyWorkers,
							humanInt(s.Sweep.Workers.ActivePeriodic), humanInt(availablePeriodic), humanInt(s.Sweep.Workers.QueuedPeriodic))
						formatLine(0, "%sBurst%s: %s active, %s available, %s queued\n", indent, specifyWorkers,
							humanInt(s.Sweep.Workers.ActiveBurst), humanInt(availableBurst), humanInt(s.Sweep.Workers.QueuedBurst))
					} else {
						formatLine(0, "%sFree%s: %s", indent, specifyWorkers, humanInt(availableFreeWorkers))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Workers stats:", "Periodic", "Burst")
						formatLine(0, "%s  %-14s %-9s %s", indent, "Active:", humanInt(s.Sweep.Workers.ActivePeriodic), humanInt(s.Sweep.Workers.ActiveBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Dedicated:", humanInt(s.Sweep.Workers.DedicatedPeriodic), humanInt(s.Sweep.Workers.DedicatedBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Available:", humanInt(availablePeriodic), humanInt(availableBurst))
						formatLine(0, "%s  %-14s %-9s %s", indent, "Queued:", humanInt(s.Sweep.Workers.QueuedPeriodic), humanInt(s.Sweep.Workers.QueuedBurst))
						formatLine(0, "%sMax connections/worker: %s", indent, humanInt(s.Sweep.Workers.MaxProvideConnsPerWorker))
						addBlankLine(0)
					}
				}
			}
			if compactMode {
				col0Width := col0MaxWidth + 2
				// Print both columns side by side
				maxRows := max(len(cols[0]), len(cols[1]))
				if maxRows == 0 {
					return nil
				}
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

// humanFloatOrNA formats a float with 1 decimal place, returning "N/A" for non-positive values.
// This is separate from humanNumberOrNA because it provides simple decimal formatting for
// continuous metrics (averages, rates) rather than SI unit formatting used for discrete counts.
func humanFloatOrNA(val float64) string {
	if val <= 0 {
		return "N/A"
	}
	return humanFull(val, 1)
}

func humanSI[T constraints.Float | constraints.Integer](val T, decimals int) string {
	v, unit := humanize.ComputeSI(float64(val))
	return fmt.Sprintf("%s%s", humanFull(v, decimals), unit)
}

func humanInt[T constraints.Integer](val T) string {
	return humanFull(float64(val), 0)
}

func humanFull(val float64, decimals int) string {
	return humanize.CommafWithDigits(val, decimals)
}

// provideCIDSync performs a synchronous/blocking provide operation to announce
// the given CID to the DHT.
//
//   - If the accelerated DHT client is used, a DHT lookup isn't needed, we
//     directly allocate provider records to closest peers.
//   - If Provide.DHT.SweepEnabled=true or OptimisticProvide=true, we make an
//     optimistic provide call.
//   - Else we make a standard provide call (much slower).
//
// IMPORTANT: The caller MUST verify DHT availability using HasActiveDHTClient()
// before calling this function. Calling with a nil or invalid router will cause
// a panic - this is the caller's responsibility to prevent.
func provideCIDSync(ctx context.Context, router routing.Routing, c cid.Cid) error {
	return router.Provide(ctx, c, true)
}
