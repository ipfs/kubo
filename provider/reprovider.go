package provider

import (
	"context"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-routing"
	"time"
)

var (
	reprovideOutgoingWorkerLimit = 8
)

// Reprovider reannounces blocks to the network
type Reprovider struct {
	ctx            context.Context
	queue          *Queue
	tracker        *Tracker
	tick           time.Duration
	initialTick    time.Duration
	blockstore     blockstore.Blockstore
	contentRouting routing.ContentRouting
	trigger        chan struct{}
}

// NewReprovider creates a reprovider that reannounces blocks that are in a tracker
//
// Reprovider periodically re-announces the cids that have been provided. These
// reprovides can be run on an interval and/or manually. Reprovider also untracks
// cids that are no longer in the blockstore.
func NewReprovider(ctx context.Context, queue *Queue, tracker *Tracker, tick time.Duration, initialTick time.Duration, blockstore blockstore.Blockstore, contentRouting routing.ContentRouting) *Reprovider {
	return &Reprovider{
		ctx:            ctx,
		queue:          queue,
		tracker:        tracker,
		tick:           tick,
		initialTick:    initialTick,
		blockstore:     blockstore,
		contentRouting: contentRouting,
		trigger:        make(chan struct{}),
	}
}

// Run starts workers to handle reprovide requests
func (rp *Reprovider) Run() {
	go rp.handleTriggers()
	go rp.handleAnnouncements()
}

// Reprovide adds all the cids in the tracker to the reprovide queue
func (rp *Reprovider) Reprovide() error {
	cids, err := rp.tracker.Tracking(rp.ctx)
	if err != nil {
		log.Warningf("error obtaining tracking information: %s", err)
		return err
	}
	for c := range cids {
		rp.queue.Enqueue(c)
	}
	return nil
}

// Trigger causes a reprovide
func (rp *Reprovider) Trigger(ctx context.Context) error {
	select {
	case <-rp.ctx.Done():
		return rp.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case rp.trigger <- struct{}{}:
	}
	return nil
}

func (rp *Reprovider) handleTriggers() {
	// dont reprovide immediately.
	// may have just started the daemon and shutting it down immediately.
	// probability( up another minute | uptime ) increases with uptime.
	after := time.After(rp.initialTick)
	for {
		if rp.tick == 0 {
			after = nil
		}

		select {
		case <-rp.ctx.Done():
			return
		case <-rp.trigger:
		case <-after:
		}

		err := rp.Reprovide()
		if err != nil {
			log.Debug(err)
		}

		after = time.After(rp.tick)
	}
}

func (rp *Reprovider) handleAnnouncements() {
	for workers := 0; workers < reprovideOutgoingWorkerLimit; workers++ {
		go func() {
			for {
				select {
				case <-rp.ctx.Done():
					return
				case c := <-rp.queue.Dequeue():
					hasBlock, err := rp.blockstore.Has(c)
					if err != nil {
						log.Warning(err)
						continue
					}
					if !hasBlock {
						rp.tracker.Untrack(c)
						continue
					}

					log.Info("reannounce - start - ", c)
					if err := rp.contentRouting.Provide(rp.ctx, c, true); err != nil {
						log.Warningf("Unable to provide entry: %s, %s", c, err)
					}
					log.Info("reannounce - end - ", c)
				}
			}
		}()
	}
}
