package gc

import (
	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("gc")

// GC performs a mark and sweep garbage collection of the blocks in the blockstore
// first, it creates a 'marked' set and adds to it the following:
// - all recursively pinned blocks, plus all of their descendants (recursively)
// - all directly pinned blocks
// - all blocks utilized internally by the pinner
//
// The routine then iterates over every block in the blockstore and
// deletes any block that is not found in the marked set.
func GC(ctx context.Context, bs bstore.GCBlockstore, pn pin.Pinner) (<-chan key.Key, error) {
	unlock := bs.GCLock()
	defer unlock()

	bsrv, err := bserv.New(bs, offline.Exchange(bs))
	if err != nil {
		return nil, err
	}
	ds := dag.NewDAGService(bsrv)

	// KeySet currently implemented in memory, in the future, may be bloom filter or
	// disk backed to conserve memory.
	gcs := key.NewKeySet()
	for _, k := range pn.RecursiveKeys() {
		gcs.Add(k)
		nd, err := ds.Get(ctx, k)
		if err != nil {
			return nil, err
		}

		// EnumerateChildren recursively walks the dag and adds the keys to the given set
		err = dag.EnumerateChildren(ctx, ds, nd, gcs)
		if err != nil {
			return nil, err
		}
	}
	for _, k := range pn.DirectKeys() {
		gcs.Add(k)
	}
	for _, k := range pn.InternalPins() {
		gcs.Add(k)

		nd, err := ds.Get(ctx, k)
		if err != nil {
			return nil, err
		}

		// EnumerateChildren recursively walks the dag and adds the keys to the given set
		err = dag.EnumerateChildren(ctx, ds, nd, gcs)
		if err != nil {
			return nil, err
		}
	}

	keychan, err := bs.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	output := make(chan key.Key)
	go func() {
		defer close(output)
		for {
			select {
			case k, ok := <-keychan:
				if !ok {
					return
				}
				if !gcs.Has(k) {
					err := bs.DeleteBlock(k)
					if err != nil {
						log.Debugf("Error removing key from blockstore: %s", err)
						return
					}
					select {
					case output <- k:
					case <-ctx.Done():
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return output, nil
}
