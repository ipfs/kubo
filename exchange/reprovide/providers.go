package reprovide

import (
	"context"

	blocks "github.com/ipfs/go-ipfs/blocks/blockstore"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmTprEaAA2A9bst5XH7exuyi5KzNMK3SEDNN8rBDnKWcUS/go-cid"
)

// NewBlockstoreProvider returns key provider using bstore.AllKeysChan
func NewBlockstoreProvider(bstore blocks.Blockstore) keyChanFunc {
	return func(ctx context.Context) (<-chan *cid.Cid, error) {
		return bstore.AllKeysChan(ctx)
	}
}

// NewPinnedProvider returns provider supplying pinned keys
func NewPinnedProvider(pinning pin.Pinner, dag merkledag.DAGService, onlyRoots bool) keyChanFunc {
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

func pinSet(ctx context.Context, pinning pin.Pinner, dag merkledag.DAGService, onlyRoots bool) (*streamingSet, error) {
	set := newStreamingSet()

	go func() {
		for _, key := range pinning.DirectKeys() {
			set.add(key)
		}

		for _, key := range pinning.RecursiveKeys() {
			set.add(key)

			if !onlyRoots {
				err := merkledag.EnumerateChildren(ctx, dag.GetLinks, key, set.add)
				if err != nil {
					return //TODO: propagate to chan / log?
				}
			}
		}

		close(set.new)
	}()

	return set, nil
}

type streamingSet struct {
	set map[string]struct{}
	new chan *cid.Cid
}

// NewSet initializes and returns a new Set.
func newStreamingSet() *streamingSet {
	return &streamingSet{
		set: make(map[string]struct{}),
		new: make(chan *cid.Cid),
	}
}

// has returns if the Set contains a given Cid.
func (s *streamingSet) has(c *cid.Cid) bool {
	_, ok := s.set[string(c.Bytes())]
	return ok
}

// add adds a Cid to the set only if it is
// not in it already.
func (s *streamingSet) add(c *cid.Cid) bool {
	if !s.has(c) {
		s.set[string(c.Bytes())] = struct{}{}
		s.new <- c
		return true
	}

	return false
}
