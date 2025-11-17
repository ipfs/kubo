package cmdenv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
	routing "github.com/libp2p/go-libp2p/core/routing"

	"github.com/ipfs/kubo/commands"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

var log = logging.Logger("core/commands/cmdenv")

// GetNode extracts the node from the environment.
func GetNode(env interface{}) (*core.IpfsNode, error) {
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

// ExecuteFastProvide immediately provides a root CID to the DHT, bypassing the regular
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
func ExecuteFastProvide(
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
	case cfg.Provide.DHT.Interval.WithDefault(config.DefaultProvideDHTInterval) == 0:
		log.Debugw("fast-provide-root: skipped", "reason", "Provide.DHT.Interval is 0")
		return nil
	case !ipfsNode.HasActiveDHTClient():
		log.Debugw("fast-provide-root: skipped", "reason", "DHT not available")
		return nil
	}

	// Check if strategy allows providing this content
	strategyStr := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)
	strategy := config.ParseProvideStrategy(strategyStr)
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

	// Asynchronous mode (default): fire-and-forget, don't block, always return nil
	log.Debugw("fast-provide-root: providing asynchronously", "cid", rootCid)
	go func() {
		// Use detached context with timeout to prevent hanging on network issues
		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultFastProvideTimeout)
		defer cancel()
		if err := provideCIDSync(ctx, ipfsNode.DHTClient, rootCid); err != nil {
			log.Warnw("fast-provide-root: async provide failed", "cid", rootCid, "error", err)
		} else {
			log.Debugw("fast-provide-root: async provide completed", "cid", rootCid)
		}
	}()
	return nil
}
