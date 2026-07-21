package ondemandpin

import (
	"context"
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

// checkTimeout prevents hung DHT query or pin/unpin operation from blocking the checker indefinitely.
const checkTimeout = 5 * time.Minute

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
		c.checkRecord(ctx, &rec)
	}
}

func (c *Checker) checkOne(ctx context.Context, ci cid.Cid) {
	rec, err := c.store.Get(ctx, ci)
	if err != nil {
		log.Debugw("CID not in store, skipping", "cid", ci, "error", err)
		return
	}
	c.checkRecord(ctx, rec)
}

// checkRecord pins below min, starts a jittered grace period above max, and
// clears grace in the deadband. Skips CIDs with a non-on-demand pin.
func (c *Checker) checkRecord(ctx context.Context, rec *Record) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	pinned, err := c.pins.IsPinned(ctx, rec.Cid)
	if err != nil {
		log.Errorw("failed to check pin state, skipping CID", "cid", rec.Cid, "error", err)
		return
	}
	hasOnDemandPin, err := c.pins.HasPinWithName(ctx, rec.Cid, OnDemandPinName)
	if err != nil {
		log.Errorw("failed to check pin name, skipping CID", "cid", rec.Cid, "error", err)
		return
	}
	if pinned && !hasOnDemandPin {
		log.Debugw("skipping: CID has a user-managed pin", "cid", rec.Cid)
		return
	}

	count, ok := CountProviders(ctx, c.routing, c.selfID, rec.Cid, c.replicationMin, c.replicationMax)
	if !ok {
		log.Warnw("provider count unknown (lookup cancelled or timed out), skipping CID", "cid", rec.Cid)
		return
	}
	log.Debugw("provider count", "cid", rec.Cid, "count", count, "min", c.replicationMin, "max", c.replicationMax, "hasOnDemandPin", hasOnDemandPin)

	switch {
	case count < c.replicationMin:
		c.handleUnderReplicated(ctx, rec, count, hasOnDemandPin)
	case count > c.replicationMax:
		c.handleWellReplicated(ctx, rec, count, hasOnDemandPin)
	default:
		c.clearGrace(ctx, rec)
	}
}

// handleUnderReplicated pins the CID if it does not already have OnDemandPinName.
func (c *Checker) handleUnderReplicated(ctx context.Context, rec *Record, count int, hasOnDemandPin bool) {
	if hasOnDemandPin {
		c.clearGrace(ctx, rec)
		return
	}

	if !c.hasStorageBudget(ctx) {
		log.Warnw("skipping pin: repo near storage limit", "cid", rec.Cid)
		return
	}

	// Re-check: a user pin may have appeared during the provider lookup.
	pinnedNow, err := c.pins.IsPinned(ctx, rec.Cid)
	if err != nil {
		log.Errorw("failed to re-check pin state before pinning, skipping CID", "cid", rec.Cid, "error", err)
		return
	}
	if pinnedNow {
		log.Debugw("skipping pin: CID gained a pin during provider lookup", "cid", rec.Cid)
		return
	}

	if err := c.pins.Pin(ctx, rec.Cid, OnDemandPinName); err != nil {
		log.Errorw("failed to pin", "cid", rec.Cid, "error", err)
		return
	}
	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	log.Infow("pinned", "cid", rec.Cid, "providers", count, "min", c.replicationMin)

	if err := c.routing.Provide(ctx, rec.Cid, true); err != nil {
		log.Warnw("failed to provide after pin", "cid", rec.Cid, "error", err)
	}
	c.saveRecord(ctx, rec)
}

// handleWellReplicated manages grace-period-then-unpin for pins with OnDemandPinName.
func (c *Checker) handleWellReplicated(ctx context.Context, rec *Record, count int, hasOnDemandPin bool) {
	if !hasOnDemandPin {
		return
	}

	if rec.LastAboveTarget.IsZero() {
		now := c.now()
		jitter := c.graceJitter()
		rec.LastAboveTarget = now
		rec.UnpinAt = now.Add(c.unpinGracePeriod + jitter)
		c.saveRecord(ctx, rec)
		log.Debugw("grace period started", "cid", rec.Cid, "providers", count, "max", c.replicationMax, "unpinAt", rec.UnpinAt, "jitter", jitter)
		return
	}

	if c.now().Before(rec.UnpinAt) {
		return
	}

	stillOnDemand, err := c.pins.HasPinWithName(ctx, rec.Cid, OnDemandPinName)
	if err != nil {
		log.Errorw("failed to check pin name, skipping unpin", "cid", rec.Cid, "error", err)
		return
	}

	if stillOnDemand {
		if err := c.pins.Unpin(ctx, rec.Cid); err != nil {
			log.Errorw("failed to unpin", "cid", rec.Cid, "error", err)
			return
		}
		log.Infow("unpinned", "cid", rec.Cid, "providers", count, "max", c.replicationMax)
	} else {
		log.Infow("relinquishing management: pin name changed externally", "cid", rec.Cid)
	}

	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	c.saveRecord(ctx, rec)
}

func (c *Checker) clearGrace(ctx context.Context, rec *Record) {
	if rec.LastAboveTarget.IsZero() && rec.UnpinAt.IsZero() {
		return
	}
	rec.LastAboveTarget = time.Time{}
	rec.UnpinAt = time.Time{}
	c.saveRecord(ctx, rec)
}

func (c *Checker) saveRecord(ctx context.Context, rec *Record) {
	if err := c.store.Update(ctx, rec); err != nil {
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
