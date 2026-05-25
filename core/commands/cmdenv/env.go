package cmdenv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/dag/walker"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/commands"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/node"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

var log = logging.Logger("core/commands/cmdenv")

// GetNode extracts the node from the environment.
func GetNode(env any) (*core.IpfsNode, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetNode()
}

// GetApi extracts CoreAPI instance from the environment.
func GetApi(env cmds.Environment, req *cmds.Request) (coreiface.CoreAPI, error) { //nolint
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	offline, _ := req.Options["offline"].(bool)
	if !offline {
		offline, _ = req.Options["local"].(bool)
		if offline {
			log.Errorf("Command '%s', --local is deprecated, use --offline instead", strings.Join(req.Path, " "))
		}
	}
	api, err := ctx.GetAPI()
	if err != nil {
		return nil, err
	}
	if offline {
		return api.WithOptions(options.Api.Offline(offline))
	}

	return api, nil
}

// GetConfigRoot extracts the config root from the environment
func GetConfigRoot(env cmds.Environment) (string, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return "", fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.ConfigRoot, nil
}

// EscNonPrint converts non-printable characters and backslash into Go escape
// sequences.  This is done to display all characters in a string, including
// those that would otherwise not be displayed or have an undesirable effect on
// the display.
func EscNonPrint(s string) string {
	if !needEscape(s) {
		return s
	}

	esc := strconv.Quote(s)
	// Remove first and last quote, and unescape quotes.
	return strings.ReplaceAll(esc[1:len(esc)-1], `\"`, `"`)
}

func needEscape(s string) bool {
	if strings.ContainsRune(s, '\\') {
		return true
	}
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
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

// ExecuteFastProvideRoot immediately provides a root CID to the DHT, bypassing the regular
// provide queue for faster content discovery. This function is reusable across commands
// that add or import content, such as ipfs add and ipfs dag import.
//
// Parameters:
//   - ctx: context for synchronous provides
//   - ipfsNode: the IPFS node instance
//   - cfg: node configuration
//   - rootCid: the CID to provide
//   - wait: whether to block until provide completes (sync mode)
//   - isPinned: whether content is pinned
//   - isPinnedRoot: whether this is a pinned root CID
//   - isMFS: whether content is in MFS
//
// Return value:
//   - Returns nil if operation succeeded or was skipped (preconditions not met)
//   - Returns error only in sync mode (wait=true) when provide operation fails
//   - In async mode (wait=false), always returns nil (errors logged in goroutine)
//
// The function handles all precondition checks (Provide.Enabled, DHT availability,
// strategy matching) and logs appropriately. In async mode, it launches a goroutine
// with a detached context and timeout.
func ExecuteFastProvideRoot(
	ctx context.Context,
	ipfsNode *core.IpfsNode,
	cfg *config.Config,
	rootCid cid.Cid,
	wait bool,
	isPinned bool,
	isPinnedRoot bool,
	isMFS bool,
) error {
	log.Debugw("fast-provide-root: enabled", "wait", wait)

	// Check preconditions for providing
	switch {
	case !cfg.Provide.Enabled.WithDefault(config.DefaultProvideEnabled):
		log.Debugw("fast-provide-root: skipped", "reason", "Provide.Enabled is false")
		return nil
	case !ipfsNode.HasActiveDHTClient():
		log.Debugw("fast-provide-root: skipped", "reason", "DHT not available")
		return nil
	}

	// Check if strategy allows providing this content
	strategyStr := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)
	strategy := config.MustParseProvideStrategy(strategyStr)
	shouldProvide := config.ShouldProvideForStrategy(strategy, isPinned, isPinnedRoot, isMFS)

	if !shouldProvide {
		log.Debugw("fast-provide-root: skipped", "reason", "strategy does not match content", "strategy", strategyStr, "pinned", isPinned, "pinnedRoot", isPinnedRoot, "mfs", isMFS)
		return nil
	}

	// Execute provide operation
	if wait {
		// Synchronous mode: block until provide completes, return error on failure
		log.Debugw("fast-provide-root: providing synchronously", "cid", rootCid)
		if err := provideCIDSync(ctx, ipfsNode.DHTClient, rootCid); err != nil {
			log.Warnw("fast-provide-root: sync provide failed", "cid", rootCid, "error", err)
			return fmt.Errorf("fast-provide: %w", err)
		}
		log.Debugw("fast-provide-root: sync provide completed", "cid", rootCid)
		return nil
	}

	// Asynchronous mode (default): fire-and-forget, don't block, always return nil.
	// Parent off the node's lifetime context (not context.Background) so the
	// goroutine cancels on daemon shutdown instead of potentially outliving
	// the node and touching a closed DHT client. The timeout still bounds
	// stuck DHT operations.
	log.Debugw("fast-provide-root: providing asynchronously", "cid", rootCid)
	go func() {
		ctx, cancel := context.WithTimeout(ipfsNode.Context(), config.DefaultFastProvideTimeout)
		defer cancel()
		if err := provideCIDSync(ctx, ipfsNode.DHTClient, rootCid); err != nil {
			log.Warnw("fast-provide-root: async provide failed", "cid", rootCid, "error", err)
		} else {
			log.Debugw("fast-provide-root: async provide completed", "cid", rootCid)
		}
	}()
	return nil
}

// ExecuteFastProvideDAG walks the DAGs rooted at roots and provides
// CIDs according to the active Provide.Strategy. A single bloom
// tracker is shared across all roots so shared sub-DAGs are
// deduplicated. Uses an unbuffered channel for backpressure.
//
// Context handling:
//   - wait=true: the walk runs inline under cmdCtx (the request
//     context), so a user Ctrl+C on the command cancels the walk.
//   - wait=false: the walk runs in a background goroutine under
//     nodeCtx (the IpfsNode lifetime context). This lets the walk
//     survive the command handler returning (go-ipfs-cmds cancels
//     req.Context on handler exit) while still being cancelled on
//     daemon shutdown, so the goroutine does not outlive the node
//     and keep the blockstore/provider pinned open.
//
// fpRate is the bloom filter target false-positive rate (1/N), normally
// resolved from cfg.Provide.BloomFPRate by the caller.
// blockCount sizes the bloom filter (pass 0 if unknown).
func ExecuteFastProvideDAG(
	cmdCtx context.Context,
	nodeCtx context.Context,
	roots []cid.Cid,
	strategy config.ProvideStrategy,
	bs blockstore.Blockstore,
	prov node.DHTProvider,
	wait bool,
	fpRate uint,
	blockCount uint,
) {
	if len(roots) == 0 {
		return
	}
	if (strategy&config.ProvideStrategyPinned) == 0 &&
		(strategy&config.ProvideStrategyMFS) == 0 {
		return
	}

	do := func(ctx context.Context) {
		expectedItems := max(uint(walker.DefaultBloomInitialCapacity), blockCount)
		tracker, err := walker.NewBloomTracker(expectedItems, fpRate)
		if err != nil {
			log.Errorf("fast-provide-dag: bloom tracker: %s", err)
			return
		}

		ch := make(chan cid.Cid) // unbuffered for backpressure
		done := make(chan struct{})
		go func() {
			defer close(done)
			for c := range ch {
				if err := prov.StartProviding(false, c.Hash()); err != nil {
					log.Errorf("fast-provide-dag: %s: %s", c, err)
				}
			}
		}()

		emit := func(c cid.Cid) bool {
			select {
			case ch <- c:
				return true
			case <-ctx.Done():
				return false
			}
		}

		opts := []walker.Option{walker.WithVisitedTracker(tracker)}
		useEntities := strategy&config.ProvideStrategyEntities != 0

		if useEntities {
			fetch := walker.NodeFetcherFromBlockstore(bs)
			for _, root := range roots {
				if ctx.Err() != nil {
					break
				}
				_ = walker.WalkEntityRoots(ctx, root, fetch, emit, opts...)
			}
		} else {
			fetch := walker.LinksFetcherFromBlockstore(bs)
			for _, root := range roots {
				if ctx.Err() != nil {
					break
				}
				_ = walker.WalkDAG(ctx, root, fetch, emit, opts...)
			}
		}

		close(ch)
		<-done
		log.Infow("fast-provide-dag: finished",
			"providedCIDs", tracker.Count(),
			"skippedBranches", tracker.Deduplicated())
	}

	if wait {
		do(cmdCtx)
	} else {
		// Use the node's lifetime context so the walk survives
		// the command handler returning (which cancels req.Context)
		// but still cancels on daemon shutdown.
		go do(nodeCtx)
	}
}
