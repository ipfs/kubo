package streamingset

import (
	"context"

	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

// StreamingSet is an extension of cid.Set which allows to implement back-pressure
// for the Visit function
type StreamingSet struct {
	Set *cid.Set
	New chan *cid.Cid
}

// NewStreamingSet initializes and returns new Set.
func NewStreamingSet() *StreamingSet {
	return &StreamingSet{
		Set: cid.NewSet(),
		New: make(chan *cid.Cid),
	}
}

// Visitor creates new visitor which adds a Cids to the set and emits them to
// the set.New channel
func (s *StreamingSet) Visitor(ctx context.Context) func(c *cid.Cid) bool {
	return func(c *cid.Cid) bool {
		if s.Set.Visit(c) {
			select {
			case s.New <- c:
			case <-ctx.Done():
			}
			return true
		}

		return false
	}
}
