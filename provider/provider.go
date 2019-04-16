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

var log = logging.Logger("provider")

const provideOutgoingWorkerLimit = 8

// Provider announces blocks to the network
type Provider interface {
	// Run is used to begin processing the provider work
	Run()
	// Provide takes a cid and makes an attempt to announce it to the network
	Provide(cid.Cid) error
	// Close stops the provider
	Close() error
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

// Close stops the provider
func (p *provider) Close() error {
	p.queue.Close()
	return nil
}

// Start workers to handle provide requests.
func (p *provider) Run() {
	p.handleAnnouncements()
}

// Provide the given cid using specified strategy.
func (p *provider) Provide(root cid.Cid) error {
	p.queue.Enqueue(root)
	return nil
}

// Handle all outgoing cids by providing (announcing) them
func (p *provider) handleAnnouncements() {
	for workers := 0; workers < provideOutgoingWorkerLimit; workers++ {
		go func() {
			for p.ctx.Err() == nil {
				select {
				case <-p.ctx.Done():
					return
				case c := <-p.queue.Dequeue():
					log.Info("announce - start - ", c)
					if err := p.contentRouting.Provide(p.ctx, c, true); err != nil {
						log.Warningf("Unable to provide entry: %s, %s", c, err)
					}
					log.Info("announce - end - ", c)
				}
			}
		}()
	}
}
