package reprovide

import (
	"context"

	pin "github.com/ipfs/go-ipfs/pin"

	merkledag "gx/ipfs/QmRiQCJZ91B7VNmLvA6sxzDuBJGSojS3uXHHVuNr3iueNZ/go-merkledag"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	blocks "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
)

// NewBlockstoreProvider returns key provider using bstore.AllKeysChan
func NewBlockstoreProvider(bstore blocks.Blockstore) KeyChanFunc {
	return func(ctx context.Context) (<-chan *cid.Cid, error) {
		return bstore.AllKeysChan(ctx)
	}
}

// NewPinnedProvider returns provider supplying pinned keys
func NewPinnedProvider(pinning pin.Pinner, dag ipld.DAGService, onlyRoots bool) KeyChanFunc {
	return func(ctx context.Context) (<-chan *cid.Cid, error) {
		set, err := pinSet(ctx, pinning, dag, onlyRoots)
		if err != nil {
			return nil, err
		}

		outCh := make(chan *cid.Cid)
		go func() {
			defer close(outCh)
			for c := range set.new {
				select {
				case <-ctx.Done():
					return
				case outCh <- c:
				}
			}

		}()

		return outCh, nil
	}
}

func pinSet(ctx context.Context, pinning pin.Pinner, dag ipld.DAGService, onlyRoots bool) (*streamingSet, error) {
	set := newStreamingSet()

	go func() {
		defer close(set.new)

		for _, key := range pinning.DirectKeys() {
			set.add(key)
		}

		for _, key := range pinning.RecursiveKeys() {
			set.add(key)

			if !onlyRoots {
				err := merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(dag), key, set.add)
				if err != nil {
					log.Errorf("reprovide indirect pins: %s", err)
					return
				}
			}
		}
	}()

	return set, nil
}

type streamingSet struct {
	set *cid.Set
	new chan *cid.Cid
}

// NewSet initializes and returns a new Set.
func newStreamingSet() *streamingSet {
	return &streamingSet{
		set: cid.NewSet(),
		new: make(chan *cid.Cid),
	}
}

// add adds a Cid to the set only if it is
// not in it already.
func (s *streamingSet) add(c *cid.Cid) bool {
	if s.set.Visit(c) {
		s.new <- c
		return true
	}

	return false
}
