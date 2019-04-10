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
	Provide(cid.Cid)
	// Close stops the provider
	Close() error
	// Tracking returns the cids being tracked by the provider system
	Tracking() (<-chan cid.Cid, error)
}

type provider struct {
	ctx context.Context
	// the CIDs for which provide announcements should be made
	queue *Queue
	// keeps track of what has been provided
	tracker *Tracker
	// used to announce providing to the network
	contentRouting routing.ContentRouting
}

// NewProvider creates a provider that announces blocks to the network using a content router
func NewProvider(ctx context.Context, queue *Queue, tracker *Tracker, contentRouting routing.ContentRouting) Provider {
	return &provider{
		ctx:            ctx,
		queue:          queue,
		tracker:        tracker,
		contentRouting: contentRouting,
	}
}

// Close stops the provider
func (p *provider) Close() error {
	return p.queue.Close()
}

// Start workers to handle provide requests.
func (p *provider) Run() {
	p.handleAnnouncements()
}

// Provide the given cid
func (p *provider) Provide(root cid.Cid) {
	p.queue.Enqueue(root)
}

func (p *provider) Tracking() (<-chan cid.Cid, error) {
	return p.tracker.Tracking(p.ctx)
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
					isTracked, err := p.tracker.IsTracking(c)
					if err != nil {
						log.Warningf("Unable to determine if tracking: %s, %s", c, err)
						continue
					}
					if isTracked {
						continue
					}

					log.Info("announce - start - ", c)
					if err := p.contentRouting.Provide(p.ctx, c, true); err != nil {
						log.Warningf("Error providing entry: %s, %s", c, err)
					}
					log.Info("announce - end - ", c)

					if err := p.tracker.Track(c); err != nil {
						log.Warningf("Unable to track entry: %s, %s", c, err)
					}
				}
			}
		}()
	}
}
