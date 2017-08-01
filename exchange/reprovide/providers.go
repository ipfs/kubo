package reprovide

import (
	"context"
	"errors"

	blocks "github.com/ipfs/go-ipfs/blocks/blockstore"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmTprEaAA2A9bst5XH7exuyi5KzNMK3SEDNN8rBDnKWcUS/go-cid"
)

func NewBlockstoreProvider(bstore blocks.Blockstore) KeyChanFunc {
	return func(ctx context.Context) (<-chan *cid.Cid, error) {
		return bstore.AllKeysChan(ctx)
	}
}

func NewPinnedProvider(pinning pin.Pinner, dag merkledag.DAGService, onlyRoots bool) KeyChanFunc {
	return func(ctx context.Context) (<-chan *cid.Cid, error) {
		set, err := pinSet(ctx, pinning, dag, onlyRoots)
		if err != nil {
			return nil, err
		}

		outCh := make(chan *cid.Cid)
		go func() {
			defer close(outCh)
			set.ForEach(func(c *cid.Cid) error {
				select {
				case <-ctx.Done():
					return errors.New("context cancelled")
				case outCh <- c:
				}
				return nil
			})
		}()

		return outCh, nil
	}
}

func pinSet(ctx context.Context, pinning pin.Pinner, dag merkledag.DAGService, onlyRoots bool) (*cid.Set, error) {
	set := cid.NewSet()
	for _, key := range pinning.DirectKeys() {
		set.Add(key)
	}

	for _, key := range pinning.RecursiveKeys() {
		set.Add(key)

		if !onlyRoots {
			err := merkledag.EnumerateChildren(ctx, dag.GetLinks, key, set.Visit)
			if err != nil {
				return nil, err
			}
		}
	}

	return set, nil
}
