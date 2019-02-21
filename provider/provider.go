// Package provider implements structures and methods to provide blocks,
// keep track of which blocks are provided, and to allow those blocks to
// be reprovided.
package provider

import (
	"context"
	"fmt"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmZBH87CAPFHcc7cYmBqeSQ98zQ3SX9KUxiYgzPmLWNVKz/go-libp2p-routing"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	"time"
)

var (
	log = logging.Logger("provider")
)

const (
	provideOutgoingWorkerLimit = 8
	provideOutgoingTimeout     = 15 * time.Second
)

type Strategy func(context.Context, cid.Cid) <-chan cid.Cid

// Provider announces blocks to the network, tracks which blocks are
// being provided, and untracks blocks when they're no longer in the blockstore.
type Provider struct {
	ctx context.Context

	// strategy for deciding which CIDs, given a CID, should be provided
	strategy Strategy
	// keeps track of which CIDs have been provided already
	tracker *Tracker
	// the CIDs for which provide announcements should be made
	queue *Queue
	// where the blocks live
	blockstore bstore.Blockstore
	// used to announce providing to the network
	contentRouting routing.ContentRouting
}

func NewProvider(ctx context.Context, strategy Strategy, tracker *Tracker, queue *Queue, blockstore bstore.Blockstore, contentRouting routing.ContentRouting) *Provider {
	return &Provider{
		ctx:            ctx,
		strategy:       strategy,
		tracker:        tracker,
		queue:          queue,
		blockstore:   	blockstore,
		contentRouting: contentRouting,
	}
}

// Start workers to handle provide requests.
func (p *Provider) Run() {
	p.handleAnnouncements()
}

// Provide the given cid using specified strategy.
func (p *Provider) Provide(root cid.Cid) error {
	cids := p.strategy(p.ctx, root)

	for cid := range cids {
		isTracking, err := p.tracker.IsTracking(cid)
		if err != nil {
			return err
		}
		if isTracking {
			continue
		}
		if err := p.queue.Enqueue(cid); err != nil {
			return err
		}
	}

	return nil
}

// Stop providing a block
func (p *Provider) Unprovide(cid cid.Cid) error {
	return p.tracker.Untrack(cid)
}

// Handle all outgoing cids by providing (announcing) them
func (p *Provider) handleAnnouncements() {
	for workers := 0; workers < provideOutgoingWorkerLimit; workers++ {
		go func() {
			for {
				select {
				case <-p.ctx.Done():
					return
				case entry := <-p.queue.Dequeue():
					// skip if already tracking
					isTracking, err := p.tracker.IsTracking(entry.cid)
					if err != nil {
						log.Warningf("Unable to check provider tracking on outgoing: %s, %s", entry.cid, err)
						continue
					}
					if isTracking {
						if err := entry.Complete(); err != nil {
							log.Warningf("Unable to complete queue entry when already tracking: %s, %s", entry.cid, err)
						}
						continue
					}

					if err := doProvide(p.ctx, p.tracker, p.blockstore, p.contentRouting, entry.cid); err != nil {
						log.Warningf("Unable to provide entry: %s, %s", entry.cid, err)
					}

					if err := entry.Complete(); err != nil {
						log.Warningf("Unable to complete queue entry when providing: %s, %s", entry.cid, err)
					}
				}
			}
		}()
	}
}

// TODO: better document this provide logic
func doProvide(ctx context.Context, tracker *Tracker, blockstore bstore.Blockstore, contentRouting routing.ContentRouting, key cid.Cid) error {
	// if not in blockstore, skip and stop tracking
	inBlockstore, err := blockstore.Has(key)
	if err != nil {
		log.Warningf("Unable to check for presence in blockstore: %s, %s", key, err)
		return err
	}
	if !inBlockstore {
		if err := tracker.Untrack(key); err != nil {
			log.Warningf("Unable to untrack: %s, %s", key, err)
			return err
		}
		return nil
	}

	// announce
	fmt.Println("announce - start - ", key)
	ctx, cancel := context.WithTimeout(ctx, provideOutgoingTimeout)
	if err := contentRouting.Provide(ctx, key, true); err != nil {
		log.Warningf("Failed to provide cid: %s", err)
		// TODO: Maybe put these failures onto a failures queue?
		cancel()
		return err
	}
	cancel()
	fmt.Println("announce - end - ", key)

	// track entry
	if err := tracker.Track(key); err != nil {
		log.Warningf("Unable to track: %s, %s", key, err)
        return err
	}

	return nil
}