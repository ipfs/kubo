// Package provider implements structures and methods to provide blocks,
// keep track of which blocks are provided, and to allow those blocks to
// be reprovided.
package provider

import (
	"context"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-routing"
)

var (
	log = logging.Logger("provider")
)

const (
	provideOutgoingWorkerLimit = 8
)

// Provider announces blocks to the network, tracks which blocks are
// being provided, and untracks blocks when they're no longer in the blockstore.
type Provider struct {
	ctx context.Context
	// the CIDs for which provide announcements should be made
	queue *Queue
	// used to announce providing to the network
	contentRouting routing.ContentRouting
}

func NewProvider(ctx context.Context, queue *Queue, contentRouting routing.ContentRouting) *Provider {
	return &Provider{
		ctx:            ctx,
		queue:          queue,
		contentRouting: contentRouting,
	}
}

// Start workers to handle provide requests.
func (p *Provider) Run() {
	p.queue.Run()
	p.handleAnnouncements()
}

// Provide the given cid using specified strategy.
func (p *Provider) Provide(root cid.Cid) error {
	return p.queue.Enqueue(root)
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
					if err := doProvide(p.ctx, p.contentRouting, entry.cid); err != nil {
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
func doProvide(ctx context.Context, contentRouting routing.ContentRouting, key cid.Cid) error {
	// announce
	log.Info("announce - start - ", key)
	if err := contentRouting.Provide(ctx, key, true); err != nil {
		log.Warningf("Failed to provide cid: %s", err)
		// TODO: Maybe put these failures onto a failures queue?
		return err
	}
	log.Info("announce - end - ", key)
	return nil
}
