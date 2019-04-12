package provider

import (
	"context"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-routing"
	"time"
)

const reprovideOutgoingWorkerLimit = 8

// Reprovider reannounces blocks to the network
type Reprovider struct {
	ctx            context.Context
	queue          *Queue
	tracker        *Tracker
	initialTick    time.Duration
	tick           time.Duration
	blockstore     blockstore.Blockstore
	contentRouting routing.ContentRouting
	trigger        chan struct{}
}

// NewReprovider creates a reprovider that reannounces blocks that are in a tracker
//
// Reprovider periodically re-announces the cids that have been provided. These
// reprovides can be run on an interval and/or manually. Reprovider also untracks
// cids that are no longer in the blockstore.
func NewReprovider(ctx context.Context, queue *Queue, tracker *Tracker, initialTick time.Duration, tick time.Duration, blockstore blockstore.Blockstore, contentRouting routing.ContentRouting) *Reprovider {
	return &Reprovider{
		ctx:            ctx,
		queue:          queue,
		tracker:        tracker,
		initialTick:    initialTick,
		tick:           tick,
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

// Close stops the reprovider
func (rp *Reprovider) Close() error {
	return rp.queue.Close()
}

// Reprovide causes a reprovide
func (rp *Reprovider) Reprovide(ctx context.Context) error {
	select {
	case <-rp.ctx.Done():
		return rp.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case rp.trigger <- struct{}{}:
	}
	return nil
}

func (rp *Reprovider) reprovide() error {
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

		err := rp.reprovide()
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
						continue
					}
					log.Info("reannounce - end - ", c)
				}
			}
		}()
	}
}
