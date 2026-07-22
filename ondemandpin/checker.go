package ondemandpin

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	peer "github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

var log = logging.Logger("ondemandpin")

// OnDemandPinName is the pin name the checker uses when creating pins (Kubo specific. Other implementation may divert from this method).
// Pins carrying this name are considered managed by on-demand pinning and may be removed automatically when replication recovers.
// The name is part of the Kubo-internal "kubo:" namespace, which ValidatePinName refuses for user-supplied
// names, so only Kubo-internal code can create pins with this name.
const OnDemandPinName = "kubo:on-demand"

// CheckTimeout bounds a single provider/pin-state lookup (checker and ls --live).
const CheckTimeout = 5 * time.Minute

type PinService interface {
	Pin(ctx context.Context, c cid.Cid, name string) error
	Unpin(ctx context.Context, c cid.Cid) error
	IsPinned(ctx context.Context, c cid.Cid) (bool, error)
	HasPinWithName(ctx context.Context, c cid.Cid, name string) (bool, error)
}

type StorageChecker interface {
	StorageUsage(ctx context.Context) (used, limit uint64, err error)
}

type Checker struct {
	store   *Store
	pins    PinService
	storage StorageChecker
	routing routing.ContentRouting
	selfID  peer.ID

	replicationMin   int
	replicationMax   int
	checkInterval    time.Duration
	unpinGracePeriod time.Duration
	maxBackoff       time.Duration
	dryRun           bool

	now         func() time.Time
	graceJitter func() time.Duration
	priorityCh  chan cid.Cid
}

func NewChecker(
	store *Store,
	pins PinService,
	storage StorageChecker,
	cr routing.ContentRouting,
	selfID peer.ID,
	cfg config.OnDemandPinning,
) *Checker {
	c := &Checker{
		store:   store,
		pins:    pins,
		storage: storage,
		routing: cr,
		selfID:  selfID,

		replicationMin:   int(cfg.ReplicationTargetMin.WithDefault(config.DefaultOnDemandPinReplicationTargetMin)),
		replicationMax:   int(cfg.ReplicationTargetMax.WithDefault(config.DefaultOnDemandPinReplicationTargetMax)),
		checkInterval:    cfg.CheckInterval.WithDefault(config.DefaultOnDemandPinCheckInterval),
		unpinGracePeriod: cfg.UnpinGracePeriod.WithDefault(config.DefaultOnDemandPinUnpinGracePeriod),
		maxBackoff:       config.DefaultOnDemandPinCheckBackoffMax,
		dryRun:           cfg.DryRun.WithDefault(false),

		now:        time.Now,
		priorityCh: make(chan cid.Cid, 64),
	}
	c.graceJitter = c.defaultGraceJitter
	return c
}

func (c *Checker) defaultGraceJitter() time.Duration {
	maxJitter := 2 * c.checkInterval
	if maxJitter <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(maxJitter)))
}

func (c *Checker) Enqueue(ci cid.Cid) {
	select {
	case c.priorityCh <- ci:
	default:
		log.Warnw("priority queue full, CID will be checked in next regular cycle", "cid", ci)
	}
}

// Run blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	log.Info("on-demand pin checker started")
	defer log.Info("on-demand pin checker stopped")

	// Warn when grace period is shorter than record validity (allowed for tests; risky on public DHT).
	if c.unpinGracePeriod < amino.DefaultProvideValidity {
		log.Warnw("UnpinGracePeriod is shorter than the DHT provider record validity; provider counts may include dead peers and this node may unpin the last live copy",
			"gracePeriod", c.unpinGracePeriod, "recordValidity", amino.DefaultProvideValidity)
	}

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ci := <-c.priorityCh:
			c.checkOne(ctx, ci)
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	records, err := c.store.List(ctx)
	if err != nil {
		log.Errorw("failed to list on-demand pins", "error", err)
		return
	}

	log.Infow("starting check cycle", "records", len(records))
	for _, rec := range records {
		// Drain priority checks between records so Enqueue'd CIDs don't wait for a full sweep to complete.
		select {
		case ci := <-c.priorityCh:
			c.checkOne(ctx, ci)
		default:
		}
		if ctx.Err() != nil {
			return
		}
		c.checkRecord(ctx, &rec, false)
	}
}

func (c *Checker) checkOne(ctx context.Context, ci cid.Cid) {
	rec, err := c.store.Get(ctx, ci)
	if err != nil {
		log.Debugw("CID not in store, skipping", "cid", ci, "error", err)
		return
	}
	// Ignore NextCheckAt so pin ondemand add is not delayed by a prior failure.
	c.checkRecord(ctx, rec, true)
}

// checkRecord pins below min, starts grace above max, clears grace in the deadband.
// immediate=true clears FailureCount/NextCheckAt before running.
// CheckTimeout covers DHT/pin-state lookup only; Pin uses ctx (daemon lifecycle).
func (c *Checker) checkRecord(ctx context.Context, rec *Record, immediate bool) {
	if immediate {
		rec.FailureCount = 0
		rec.NextCheckAt = time.Time{}
	} else if !rec.NextCheckAt.IsZero() && c.now().Before(rec.NextCheckAt) {
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()

	pinned, err := c.pins.IsPinned(lookupCtx, rec.Cid)
	if err != nil {
		c.recordFailure(ctx, rec, fmt.Errorf("check pin state: %w", err))
		return
	}
	hasOnDemandPin, err := c.pins.HasPinWithName(lookupCtx, rec.Cid, OnDemandPinName)
	if err != nil {
		c.recordFailure(ctx, rec, fmt.Errorf("check pin name: %w", err))
		return
	}
	if pinned && !hasOnDemandPin {
		log.Debugw("skipping: CID has a user-managed pin", "cid", rec.Cid)
		rec.LastResult = "user-pin"
		c.clearBackoff(ctx, rec)
		return
	}

	count, ok := CountProviders(lookupCtx, c.routing, c.selfID, rec.Cid, c.replicationMin, c.replicationMax)
	if !ok {
		rec.LastResult = "lookup-unknown"
		c.recordFailure(ctx, rec, fmt.Errorf("provider count unknown"))
		return
	}
	log.Debugw("provider count", "cid", rec.Cid, "count", count, "min", c.replicationMin, "max", c.replicationMax, "hasOnDemandPin", hasOnDemandPin)

	switch {
	case count < c.replicationMin:
		if err := c.handleUnderReplicated(ctx, lookupCtx, rec, count, hasOnDemandPin); err != nil {
			c.recordFailure(ctx, rec, err)
			return
		}
	case count > c.replicationMax:
		if err := c.handleWellReplicated(ctx, lookupCtx, rec, count, hasOnDemandPin); err != nil {
			c.recordFailure(ctx, rec, err)
			return
		}
	default:
		c.clearGrace(ctx, rec)
		rec.LastResult = "deadband"
	}
	rec.LastProviderCount = count
	c.clearBackoff(ctx, rec)
}

// handleUnderReplicated pins the CID if it does not already have OnDemandPinName.
// lookupCtx is for quick pin-state checks; runCtx is for Pin/Provide/store (may outlast CheckTimeout).
func (c *Checker) handleUnderReplicated(runCtx, lookupCtx context.Context, rec *Record, count int, hasOnDemandPin bool) error {
	if hasOnDemandPin {
		c.clearGrace(runCtx, rec)
		rec.LastResult = "holding"
		return nil
	}

	if !c.hasStorageBudget(runCtx) {
		log.Warnw("skipping pin: repo near storage limit", "cid", rec.Cid)
		rec.LastResult = "storage-limit"
		return nil
	}

	// Re-check: a user pin may have appeared during the provider lookup.
	pinnedNow, err := c.pins.IsPinned(lookupCtx, rec.Cid)
	if err != nil {
		return fmt.Errorf("re-check pin state: %w", err)
	}
	if pinnedNow {
		log.Debugw("skipping pin: CID gained a pin during provider lookup", "cid", rec.Cid)
		rec.LastResult = "user-pin"
		return nil
	}

	if c.dryRun {
		log.Infow("dry-run: would pin", "cid", rec.Cid, "providers", count, "min", c.replicationMin)
		rec.LastResult = "would-pin"
		return nil
	}

	if err := c.pins.Pin(runCtx, rec.Cid, OnDemandPinName); err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	rec.LastResult = "pinned"
	log.Infow("pinned", "cid", rec.Cid, "providers", count, "min", c.replicationMin)

	if err := c.routing.Provide(runCtx, rec.Cid, true); err != nil {
		log.Warnw("failed to provide after pin", "cid", rec.Cid, "error", err)
	}
	c.saveRecord(runCtx, rec)
	return nil
}

func (c *Checker) handleWellReplicated(runCtx, lookupCtx context.Context, rec *Record, count int, hasOnDemandPin bool) error {
	if !hasOnDemandPin {
		rec.LastResult = "above-max"
		return nil
	}

	if rec.LastAboveTarget.IsZero() {
		now := c.now()
		jitter := c.graceJitter()
		rec.LastAboveTarget = now
		rec.UnpinAt = now.Add(c.unpinGracePeriod + jitter)
		rec.LastResult = "grace"
		c.saveRecord(runCtx, rec)
		log.Debugw("grace period started", "cid", rec.Cid, "providers", count, "max", c.replicationMax, "unpinAt", rec.UnpinAt, "jitter", jitter)
		return nil
	}

	if c.now().Before(rec.UnpinAt) {
		rec.LastResult = "grace"
		return nil
	}

	stillOnDemand, err := c.pins.HasPinWithName(lookupCtx, rec.Cid, OnDemandPinName)
	if err != nil {
		return fmt.Errorf("check pin name before unpin: %w", err)
	}

	if stillOnDemand {
		if c.dryRun {
			log.Infow("dry-run: would unpin", "cid", rec.Cid, "providers", count, "max", c.replicationMax)
			rec.LastResult = "would-unpin"
			return nil
		}
		if err := c.pins.Unpin(runCtx, rec.Cid); err != nil {
			return fmt.Errorf("unpin: %w", err)
		}
		log.Infow("unpinned", "cid", rec.Cid, "providers", count, "max", c.replicationMax)
		rec.LastResult = "unpinned"
	} else {
		log.Infow("relinquishing management: pin name changed externally", "cid", rec.Cid)
		rec.LastResult = "released"
	}

	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	c.saveRecord(runCtx, rec)
	return nil
}

func (c *Checker) clearGrace(ctx context.Context, rec *Record) {
	if rec.LastAboveTarget.IsZero() && rec.UnpinAt.IsZero() {
		return
	}
	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	c.saveRecord(ctx, rec)
}

func (c *Checker) clearBackoff(ctx context.Context, rec *Record) {
	rec.LastCheckedAt = c.now()
	rec.FailureCount = 0
	rec.NextCheckAt = time.Time{}
	c.saveRecord(ctx, rec)
}

func (c *Checker) recordFailure(ctx context.Context, rec *Record, cause error) {
	rec.LastCheckedAt = c.now()
	if rec.LastResult == "" {
		rec.LastResult = "error"
	}
	rec.FailureCount++
	delay := c.backoffDelay(rec.FailureCount)
	rec.NextCheckAt = c.now().Add(delay)
	log.Warnw("check failed", "cid", rec.Cid, "error", cause, "failures", rec.FailureCount, "nextCheckAt", rec.NextCheckAt)
	c.saveRecord(ctx, rec)
}

func (c *Checker) backoffDelay(failures int) time.Duration {
	if failures < 1 {
		return c.checkInterval
	}
	d := c.checkInterval
	for i := 1; i < failures; i++ {
		if d >= c.maxBackoff/2 {
			return c.maxBackoff
		}
		d *= 2
	}
	if d > c.maxBackoff {
		return c.maxBackoff
	}
	return d
}

func (c *Checker) saveRecord(ctx context.Context, rec *Record) {
	if err := c.store.Update(ctx, rec); err != nil {
		if errors.Is(err, ErrNotRegistered) {
			log.Debugw("record gone during check, not recreating", "cid", rec.Cid)
			return
		}
		log.Errorw("failed to update record", "cid", rec.Cid, "error", err)
	}
}

func (c *Checker) hasStorageBudget(ctx context.Context) bool {
	if c.storage == nil {
		return true
	}
	used, limit, err := c.storage.StorageUsage(ctx)
	if err != nil {
		log.Warnw("failed to check storage usage, proceeding with pin", "error", err)
		return true
	}
	if limit == 0 {
		return true
	}
	return used < limit
}

// CountProviders counts providers excluding self. Asks for max+2 results so
// self can take a slot and we can still see max+1 others.
// ok is false if the lookup was cancelled before reaching min providers.
func CountProviders(ctx context.Context, cr routing.ContentRouting, selfID peer.ID, c cid.Cid, min, max int) (count int, ok bool) {
	ch := cr.FindProvidersAsync(ctx, c, max+2)

	seen := make(map[peer.ID]struct{})
	for pi := range ch {
		if pi.ID == selfID {
			continue
		}
		seen[pi.ID] = struct{}{}
	}
	count = len(seen)
	if ctx.Err() != nil && count < min {
		return count, false
	}
	return count, true
}

// PinHasName is used by checker (via PinService.HasPinWithName) and the rm command to identify pins managed by on-demand pinning.
func PinHasName(ctx context.Context, p pin.Pinner, c cid.Cid, name string) (bool, error) {
	results, err := p.CheckIfPinnedWithType(ctx, pin.Recursive, true, c)
	if err != nil {
		return false, err
	}
	for _, r := range results {
		if r.Pinned() && r.Name == name {
			return true, nil
		}
	}
	return false, nil
}
