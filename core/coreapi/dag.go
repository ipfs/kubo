package coreapi

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/files"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	pin "github.com/ipfs/boxo/pinning/pinner"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	"github.com/ipfs/kubo/config"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	gocarv2 "github.com/ipld/go-car/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/kubo/tracing"
)

type dagAPI struct {
	ipld.DAGService

	core *CoreAPI
}

type pinningAdder CoreAPI

func (adder *pinningAdder) Add(ctx context.Context, nd ipld.Node) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinningAdder", "Add", trace.WithAttributes(attribute.String("node", nd.String())))
	defer span.End()
	defer adder.blockstore.PinLock(ctx).Unlock(ctx)

	if err := adder.dag.Add(ctx, nd); err != nil {
		return err
	}

	if err := adder.pinning.PinWithMode(ctx, nd.Cid(), pin.Recursive, ""); err != nil {
		return err
	}

	return adder.pinning.Flush(ctx)
}

func (adder *pinningAdder) AddMany(ctx context.Context, nds []ipld.Node) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinningAdder", "AddMany", trace.WithAttributes(attribute.Int("nodes.count", len(nds))))
	defer span.End()
	defer adder.blockstore.PinLock(ctx).Unlock(ctx)

	if err := adder.dag.AddMany(ctx, nds); err != nil {
		return err
	}

	cids := cid.NewSet()

	for _, nd := range nds {
		c := nd.Cid()
		if cids.Visit(c) {
			if err := adder.pinning.PinWithMode(ctx, c, pin.Recursive, ""); err != nil {
				return err
			}
		}
	}

	return adder.pinning.Flush(ctx)
}

func (api *dagAPI) Pinning() ipld.NodeAdder {
	return (*pinningAdder)(api.core)
}

func (api *dagAPI) Session(ctx context.Context) ipld.NodeGetter {
	return dag.NewSession(ctx, api.DAGService)
}

func (api *dagAPI) Import(ctx context.Context, file files.File, opts ...options.DagImportOption) (<-chan iface.DagImportResult, error) {
	// Parse options
	settings, err := options.DagImportOptions(opts...)
	if err != nil {
		return nil, err
	}

	// Get config for batch settings
	cfg, err := api.core.repo.Config()
	if err != nil {
		return nil, err
	}

	// Create block decoder for IPLD nodes
	blockDecoder := ipldlegacy.NewDecoder()

	// Create batch for efficient block addition
	// Uses config values for batch size tuning
	batch := ipld.NewBatch(ctx, api.DAGService,
		ipld.MaxNodesBatchOption(int(cfg.Import.BatchMaxNodes.WithDefault(config.DefaultBatchMaxNodes))),
		ipld.MaxSizeBatchOption(int(cfg.Import.BatchMaxSize.WithDefault(config.DefaultBatchMaxSize))),
	)

	// Create output channel
	out := make(chan iface.DagImportResult)

	// Process import in background
	go func() {
		defer close(out)
		defer file.Close()

		// Acquire pinlock if pinning roots (also serves as GC lock)
		if settings.PinRoots {
			unlocker := api.core.blockstore.PinLock(ctx)
			defer unlocker.Unlock(ctx)
		}

		// Track roots from CAR headers and stats
		roots := cid.NewSet()
		var blockCount, blockBytesCount uint64

		// Parse CAR file
		car, err := gocarv2.NewBlockReader(file)
		if err != nil {
			out <- iface.DagImportResult{Err: fmt.Errorf("failed to create CAR reader: %w", err)}
			return
		}

		// Collect roots from CAR header
		for _, c := range car.Roots {
			roots.Add(c)
		}

		// Process all blocks from CAR file
		var previous blocks.Block
		for {
			block, err := car.Next()
			if err != nil {
				if err != io.EOF {
					if previous != nil {
						out <- iface.DagImportResult{Err: fmt.Errorf("error reading block after %s: %w", previous.Cid(), err)}
					} else {
						out <- iface.DagImportResult{Err: fmt.Errorf("error reading CAR blocks: %w", err)}
					}
				}
				break
			}
			if block == nil {
				break
			}

			// Decode block into IPLD node
			nd, err := blockDecoder.DecodeNode(ctx, block)
			if err != nil {
				out <- iface.DagImportResult{Err: fmt.Errorf("failed to decode block %s: %w", block.Cid(), err)}
				return
			}

			// Add node to batch
			if err := batch.Add(ctx, nd); err != nil {
				out <- iface.DagImportResult{Err: fmt.Errorf("failed to add block %s to batch: %w", nd.Cid(), err)}
				return
			}

			blockCount++
			blockBytesCount += uint64(len(block.RawData()))
			previous = block

			// Check context cancellation
			select {
			case <-ctx.Done():
				out <- iface.DagImportResult{Err: ctx.Err()}
				return
			default:
			}
		}

		// Commit batch to blockstore
		if err := batch.Commit(); err != nil {
			out <- iface.DagImportResult{Err: fmt.Errorf("failed to commit batch: %w", err)}
			return
		}

		// Emit all roots (with pin status if requested)
		err = roots.ForEach(func(c cid.Cid) error {
			result := iface.DagImportResult{
				Root: &iface.DagImportRoot{Cid: c},
			}

			// Attempt to pin if requested
			if settings.PinRoots {
				// Verify block exists in blockstore
				block, err := api.core.blockstore.Get(ctx, c)
				if err != nil {
					result.Root.PinErrorMsg = fmt.Sprintf("blockstore get: %v", err)
				} else {
					// Decode node for pinning
					nd, err := blockDecoder.DecodeNode(ctx, block)
					if err != nil {
						result.Root.PinErrorMsg = fmt.Sprintf("decode node: %v", err)
					} else {
						// Pin recursively
						err = api.core.pinning.Pin(ctx, nd, true, "")
						if err != nil {
							result.Root.PinErrorMsg = fmt.Sprintf("pin: %v", err)
						} else {
							// Flush pins to storage
							err = api.core.pinning.Flush(ctx)
							if err != nil {
								result.Root.PinErrorMsg = fmt.Sprintf("flush: %v", err)
							}
						}
					}
				}
			}

			// Send root result
			select {
			case out <- result:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})
		if err != nil {
			out <- iface.DagImportResult{Err: fmt.Errorf("error emitting roots: %w", err)}
			return
		}

		// Emit stats if requested
		if settings.Stats {
			select {
			case out <- iface.DagImportResult{
				Stats: &iface.DagImportStats{
					BlockCount:      blockCount,
					BlockBytesCount: blockBytesCount,
				},
			}:
			case <-ctx.Done():
				return
			}
		}

		// Execute fast-provide (will check if enabled)
		if err := api.executeFastProvide(ctx, cfg, roots, settings.FastProvideRoot, settings.FastProvideWait, settings.PinRoots, settings.PinRoots, false); err != nil {
			select {
			case out <- iface.DagImportResult{Err: err}:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}

// executeFastProvide announces roots to the DHT for faster discovery
func (api *dagAPI) executeFastProvide(ctx context.Context, cfg *config.Config, roots *cid.Set, enabled bool, wait bool, isPinned bool, isPinnedRoot bool, isMFS bool) error {
	// Check if fast-provide is enabled
	if !enabled {
		if wait {
			log.Debugw("fast-provide-root: skipped", "reason", "disabled by flag or config", "wait-flag-ignored", true)
		} else {
			log.Debugw("fast-provide-root: skipped", "reason", "disabled by flag or config")
		}
		return nil
	}

	log.Debugw("fast-provide-root: enabled", "wait", wait)

	// Check preconditions for providing
	if !cfg.Provide.Enabled.WithDefault(config.DefaultProvideEnabled) {
		log.Debugw("fast-provide-root: skipped", "reason", "Provide.Enabled is false")
		return nil
	}

	if cfg.Provide.DHT.Interval.WithDefault(config.DefaultProvideDHTInterval) == 0 {
		log.Debugw("fast-provide-root: skipped", "reason", "Provide.DHT.Interval is 0")
		return nil
	}

	if !api.core.nd.HasActiveDHTClient() {
		log.Debugw("fast-provide-root: skipped", "reason", "DHT not available")
		return nil
	}

	// Check provide strategy
	strategyStr := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)
	strategy := config.ParseProvideStrategy(strategyStr)
	shouldProvide := config.ShouldProvideForStrategy(strategy, isPinned, isPinnedRoot, isMFS)

	if !shouldProvide {
		log.Debugw("fast-provide-root: skipped", "reason", "strategy does not match content", "strategy", strategyStr, "pinned", isPinned, "pinnedRoot", isPinnedRoot, "mfs", isMFS)
		return nil
	}

	// Provide each root
	return roots.ForEach(func(c cid.Cid) error {
		if wait {
			// Synchronous mode: block until provide completes
			log.Debugw("fast-provide-root: providing synchronously", "cid", c)
			if err := api.core.nd.DHTClient.Provide(ctx, c, true); err != nil {
				log.Warnw("fast-provide-root: sync provide failed", "cid", c, "error", err)
				return fmt.Errorf("fast-provide: %w", err)
			}
			log.Debugw("fast-provide-root: sync provide completed", "cid", c)
		} else {
			// Asynchronous mode: fire-and-forget in goroutine
			log.Debugw("fast-provide-root: providing asynchronously", "cid", c)
			go func(rootCid cid.Cid) {
				// Use detached context with timeout to prevent hanging
				asyncCtx, cancel := context.WithTimeout(context.Background(), config.DefaultFastProvideTimeout)
				defer cancel()
				if err := api.core.nd.DHTClient.Provide(asyncCtx, rootCid, true); err != nil {
					log.Warnw("fast-provide-root: async provide failed", "cid", rootCid, "error", err)
				} else {
					log.Debugw("fast-provide-root: async provide completed", "cid", rootCid)
				}
			}(c)
		}
		return nil
	})
}

var (
	_ ipld.DAGService  = (*dagAPI)(nil)
	_ dag.SessionMaker = (*dagAPI)(nil)
)
