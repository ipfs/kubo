// Package provider implements structures and methods to provide blocks,
// keep track of which blocks are provided, and to allow those blocks to
// be reprovided.
package provider

import (
	"context"

	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	routing "github.com/libp2p/go-libp2p-routing"
)

var (
	log = logging.Logger("provider")
)

const (
	provideOutgoingWorkerLimit = 8
)

// Provider announces blocks to the network
type Provider interface {
	Run()
	Provide(cid.Cid) error
}

type provider struct {
	ctx context.Context
	// the CIDs for which provide announcements should be made
	queue *Queue
	// used to announce providing to the network
	contentRouting routing.ContentRouting
}

// NewProvider creates a provider that announces blocks to the network using a content router
func NewProvider(ctx context.Context, queue *Queue, contentRouting routing.ContentRouting) Provider {
	return &provider{
		ctx:            ctx,
		queue:          queue,
		contentRouting: contentRouting,
	}
}

// Start workers to handle provide requests.
func (p *provider) Run() {
	p.queue.Run()
	p.handleAnnouncements()
}

// Provide the given cid using specified strategy.
func (p *provider) Provide(root cid.Cid) error {
	return p.queue.Enqueue(root)
}

// Handle all outgoing cids by providing (announcing) them
func (p *provider) handleAnnouncements() {
	for workers := 0; workers < provideOutgoingWorkerLimit; workers++ {
		go func() {
			for {
				select {
				case <-p.ctx.Done():
					return
				case entry := <-p.queue.Dequeue():
					log.Info("announce - start - ", entry.cid)
					if err := p.contentRouting.Provide(p.ctx, entry.cid, true); err != nil {
						log.Warningf("Unable to provide entry: %s, %s", entry.cid, err)
					}
					log.Info("announce - end - ", entry.cid)
				}
			}
		}()
	}
}
